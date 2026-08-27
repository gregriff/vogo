//go:build cgo

package netw

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"golang.org/x/net/websocket"
	"golang.org/x/sync/errgroup"
)

func JoinChannel(ctx context.Context, creds *credentials, ownerName, channelName string) error {
	// TODO:
	// - note: later, requests.BulkConnection could be parallized, and the GUI could use recent status polling to
	//         issue offers ahead of time, cancelling them if joinRoom returns that the user is no longer in the room
	// pseudocode:
	// - make sure to send connection successful sentinels

	track := wrtc.CreateAudioTrack(creds.username)
	channel := audio.NewChannel(track, wrtc.RecvMTU)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return channel.CreateDeviceContext(gCtx)
	})
	defer func() {
		if err := channel.Uninit(); err != nil {
			log.Printf("error uninitializing allocated ctx: %v", err)
		}
	}()

	// todo: defer a func that uninits the context once, waiting for both devices to be uninited on two chans.
	g.Go(func() error {
		return channel.Speaker.Init(gCtx, channel.DataProc())
	})
	defer channel.Speaker.Uninit()

	g.Go(func() error {
		return channel.Mic.Init(gCtx)
	})
	defer channel.Mic.Uninit()

	g.Go(func() error {
		return joinChannelAndConnect(gCtx, creds, ownerName, channelName, channel)
	})

	// start speaker and mic once both are initialized
	g.Go(func() error {
		micReady := channel.Mic.Initialized()
		speakerReady := channel.Speaker.Initialized()

		for {
			select {
			case <-gCtx.Done():
				return nil
			case <-speakerReady:
				// for testing, disable speaker for test user two
				// NOTE: if two speakers are playing on the same machine, audio will sound bad
				// NOTE: if Speaker.Start() is not called, memory leaks occur.
				// if user := os.Getenv("VOGOENV"); user == "two" {
				// 	speakerReady = nil
				// }
				if err := channel.Speaker.Start(); err != nil {
					return err
				}
				speakerReady = nil
			case <-micReady:
				micReady = nil
			}

			// Both initialized
			if micReady == nil && speakerReady == nil {
				return channel.Mic.Start(gCtx)
			}
		}
	})

	return g.Wait()
}

func joinChannelAndConnect(
	ctx context.Context,
	creds *credentials,
	ownerName, channelName string,
	audioState *audio.Channel,
) error {
	ws, err := newWebsocket(ctx, creds, "/channel/join")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer func() {
		_ = closeAndWait(ws, nil)
	}()

	req := requests.JoinChannel{RoomName: channelName, OwnerName: ownerName}
	if err = websocket.JSON.Send(ws, req); err != nil {
		return fmt.Errorf("error sending join channel request: %w", err)
	}

	// contains users to send offers to
	var res requests.BulkConnection
	if err := wsock.ReceiveJSON(ctx, ws, &res); err != nil {
		return fmt.Errorf("error reading channel users from ws: %v", err)
	}

	// TODO: short circuit here, only sending blank bulkmsg and running msg loop
	if len(res.Users) == 0 {
		log.Println("no users in channel")
	}

	// this holds the state of the room from this client's perspective
	room := InitRoom(ws, audioState, creds)
	defer room.Uninit()
	for id, name := range res.Users {
		log.Printf("creating new connection with %s", name)
		room.AddConnection(ctx, id, name)
	}

	// create all offers and send in bulk to server
	start := time.Now()
	bulkReq := messages.BulkConnection{Data: make(map[uuid.UUID]requests.Connection, shared.ChannelCapacity-1)}
	for recipient, c := range room.Snapshot() {
		req := c.NewOffer(creds.username, recipient)
		bulkReq.Data[c.Id] = req
	}
	log.Printf("offers took %v to generate\n", time.Since(start))

	if err = websocket.JSON.Send(ws, bulkReq); err != nil {
		return fmt.Errorf("error sending bulk offers: %w", err)
	}

	room.SendInitialCandidates(ctx)

	g, gCtx := errgroup.WithContext(ctx)
	defer func() {
		_ = closeAndWait(ws, g)
	}()

	msgs := make(chan wsock.Message)
	g.Go(func() error {
		var err error
		if err = wsock.Listen(gCtx, ws, msgs); err != nil {
			if err == io.EOF { // todo: may need to handle this in startMessageLoop
				err = fmt.Errorf("closed by server")
			}
			_ = ws.WriteClose(1)
		}
		return err
	})

	// this should run until the client leaves the room. it needs to handle
	// completing pending connections and also future connections when new users join the room.
	for {
		select {
		case <-ctx.Done():
			return g.Wait()
		case msg := <-msgs:
			room.HandleMessage(ctx, msg)
		}
	}
}
