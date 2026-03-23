//go:build cgo

package netw

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/netw/wrtc"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
	"golang.org/x/sync/errgroup"
)

// CallFriend creates a bidirectional voice call to the intended recipient.
// Signaling, speaker init, connecting and microphone init are all run concurrently,
// organized with waitgroups and synchronized with channels. The entire process can
// be cancelled with the provided context, and the first error encountered will be returned.
func CallFriend(ctx context.Context, creds *credentials, recipient string) error {
	track := wrtc.CreateAudioTrack(creds.username)
	conn := NewConnection(uuid.New(), recipient, creds.stunServer, track)
	call := audio.NewCall(track, wrtc.RecvMTU)
	call.AddPeer(conn.Pc)
	defer conn.Close()

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return call.CreateDeviceContext(gCtx)
	})
	defer func() {
		if err := call.Uninit(); err != nil {
			log.Printf("error uninitializing allocated ctx: %v", err)
		}
	}()

	g.Go(func() error {
		return call.Speaker.Init(gCtx, call.DataProc())
	})
	defer func() {
		conn.Close()
		call.Speaker.Uninit()
	}()

	g.Go(func() error {
		return call.Mic.Init(gCtx)
	})
	defer call.Mic.Uninit()

	g.Go(func() error {
		return sendCallAndConnect(gCtx, conn, creds, recipient)
	})

	// this will return io.EOF when PC fails.
	g.Go(func() error {
		return conn.HandleEvents(gCtx, recipient)
	})

	g.Go(func() error {
		return conn.CollectSenderReports()
	})

	g.Go(func() error {
		return conn.CollectReceiverReports()
	})

	// init microphone, and start it and the speaker once call is connected
	g.Go(func() error {
		micReady := call.Mic.Initialized()
		speakerReady := call.Speaker.Initialized()

		for {
			select {
			case <-gCtx.Done():
				return nil
			case <-conn.Connected:
				conn.Connected = nil
			case <-speakerReady:
				// for testing, disable speaker for caller
				// if err := audioState.Speaker.Start(); err != nil {
				// 	return err
				// }
				speakerReady = nil
			case <-micReady:
				micReady = nil
			}

			// Both initialized and call connected
			if micReady == nil && speakerReady == nil && conn.Connected == nil {
				return call.Mic.Start(gCtx)
			}
		}
	})

	return g.Wait()
}

// sendCallAndConnect creates and establishes a voice call with a friend client, if
// they answer the call. It uses a websocket connection to a vogo server to handle
// signaling and connecting, and uses trickle-ICE for fast connection. It assumes
// a PeerConnection set up correctly for opus audio.
func sendCallAndConnect(
	ctx context.Context,
	conn *Connection,
	creds *credentials,
	recipient string,
) error {
	ws, err := newWebsocket(ctx, creds, "/call")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}
	defer func() {
		_ = closeAndWait(ws, nil)
	}()

	// send offer
	offer := conn.NewOffer(creds.username, recipient)
	if err = websocket.JSON.Send(ws, offer); err != nil {
		return fmt.Errorf("error sending offer: %w", err)
	}

	g, gCtx := errgroup.WithContext(ctx)
	defer func() {
		_ = closeAndWait(ws, g)
	}()

	// gather local ice candidates and write to websocket
	g.Go(func() error {
		return sendCandidates(gCtx, ws, conn, creds.username, wsock.ICEOffer)
	})

	// wait to recv answer
	var answer webrtc.SessionDescription
	if err := wsock.ReceiveJSON(gCtx, ws, &answer); err != nil {
		return fmt.Errorf("error reading answer from ws: %v", err)
	}
	if err := conn.Pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("error while setting remote description: %w", err)
	}
	log.Println("received answer")

	// recv recipient candidates
	g.Go(func() error {
		return recvCandidates(gCtx, ws, conn.Pc)
	})

	<-conn.Connected
	g.Go(func() error {
		return notifyConnected(ws, creds.username, recipient)
	})
	return g.Wait()
}
