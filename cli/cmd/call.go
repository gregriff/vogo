package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gregriff/vogo/cli/configs"
	"github.com/gregriff/vogo/cli/internal"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/cli/internal/services/signaling"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/websocket"
	// _ "net/http/pprof".
)

var callCmd = &cobra.Command{
	Use:   "call",
	Short: "Call a friend",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		username, password := viper.GetString("user.name"), viper.GetString("user.password")
		if len(username) == 0 {
			return fmt.Errorf("username not found. ensure it is present in %s", ConfigFile)
		}
		if len(password) == 0 {
			return fmt.Errorf("password not found. ensure it is present in %s", ConfigFile)
		}

		if len(args) == 0 {
			return fmt.Errorf("recipient must be specified as an argument")
		}

		recipient := args[0]
		if len(recipient) > 16 {
			return fmt.Errorf("recipient string too long")
		}
		viper.Set("recipient", recipient)
		return nil
	},
	Run: initiateCall,
}

func init() {
	rootCmd.AddCommand(callCmd)
}

func initiateCall(_ *cobra.Command, _ []string) {
	_, vogoServer, stunServer, recipient, username, password := viper.GetBool("debug"),
		viper.GetString("servers.vogo-origin"),
		viper.GetString("servers.stun-origin"),
		viper.GetString("recipient"),
		viper.GetString("user.name"),
		viper.GetString("user.password")

	// parent context for all other contexts to be derived from
	sigCtx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("creating caller connection")
	pc, err := configs.NewPeerConnection(stunServer)
	if err != nil {
		fmt.Printf("error creating peer connection %v", err)
		return
	}
	defer func() {
		log.Println("forcing close of caller connection")
		if err := pc.Close(); err != nil {
			fmt.Printf("cannot forcefully close caller connection: %v\n", err)
		}
	}()

	track, err := configs.CreateAudioTrack(pc, username)
	if err != nil {
		log.Printf("error creating audio track: %v", err)
		return
	}

	// var playbackWg sync.WaitGroup

	// playbackCtx, device, playbackErr := audio.SetupPlayback(pc, &playbackWg)
	// defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	// if playbackErr != nil {
	// 	fmt.Printf("error initializing playback system: %v", playbackErr)
	// 	return
	// }

	var (
		// our client's ice candidates
		candidates = make(chan webrtc.ICECandidateInit, 10)
		connected  = make(chan struct{})
	)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		internal.OnICECandidate(c, candidates)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		internal.OnConnectionStateChangeCaller(s, connected)
	})

	// sending an error on this channel will abort the call process
	errorChan := make(chan error, 10)

	var callWg sync.WaitGroup
	callCtx, cancelCall := context.WithCancel(sigCtx)
	defer func() {
		cancelCall()
		callWg.Wait()
		log.Println("call wg completed")
	}()

	callWg.Go(func() {
		defer cancelCall()

		credentials := signaling.NewCredentials(vogoServer, username, password)
		err := sendCallAndConnect(callCtx, pc, credentials, recipient, candidates, errorChan)
		if err != nil {
			errorChan <- err
			return
		}
	})

	var captureWg sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(sigCtx)
	defer func() {
		cancelCapture()
		captureWg.Wait()
	}()

	// setup microphone once call is connected and capture until cancelled
	captureWg.Go(func() {
		select {
		case <-captureCtx.Done(): // if call fails
			return
		case <-connected:
			cancelCall()
			break
		}
		if err = audio.StartCapture(captureCtx, pc, track); err != nil {
			errorChan <- fmt.Errorf("error with capture device: %w", err)
			return
		}
	})

	// block until sigint or error in goroutines above
	select {
	case err := <-errorChan:
		fmt.Println(err)
		break
	case <-sigCtx.Done():
		break
	}
}

// sendCallAndConnect creates and establishes a voice call with a friend client, if
// they answer the call. It uses a websocket connection to a vogo server to handle
// signaling and connecting, and uses trickle-ICE for fast connection.
func sendCallAndConnect(
	ctx context.Context,
	pc *webrtc.PeerConnection,
	credentials *signaling.Credentials,
	recipient string,
	candidates <-chan webrtc.ICECandidateInit,
	abort chan<- error,
) error {
	ws, err := signaling.NewConnection(ctx, credentials, "/call")
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}

	if err = signaling.CreateAndSendOffer(ws, pc, recipient); err != nil {
		return err
	}

	var iceWg sync.WaitGroup
	iceCtx, cancelSendIce := context.WithCancel(ctx)
	defer func() {
		cancelSendIce()
		iceWg.Wait()
	}()

	// gather local ice candidates and write to websocket
	iceWg.Go(func() {
		defer cancelSendIce()
		if err = sendCandidates(iceCtx, ws, candidates); err != nil {
			abort <- err // this will cause surrounding function to cancel
		}
	})

	// wait to recv answer
	var answer webrtc.SessionDescription
	if err = websocket.JSON.Receive(ws, &answer); err != nil {
		return fmt.Errorf("error reading answer from ws: %v", err)
	}
	if err = pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("error while setting remote description: %w", err)
	}
	log.Println("recieved answer")

	var (
		readWg              sync.WaitGroup
		recipientCandidates = make(chan webrtc.ICECandidateInit)
	)
	defer signaling.CloseAndWait(ws, &readWg)

	// read recipient candidates from ws as they come in
	readWg.Go(func() {
		signaling.ReadCandidates(ws, recipientCandidates)
	})
	if err = addCandidates(ctx, pc, recipientCandidates); err != nil {
		return err
	}
	return nil
}

// sendCandidates sends the caller's ICE candidates from ch to the websocket as they're gathered.
// It returns when there are no more candidates or the context is cancelled.
func sendCandidates(ctx context.Context, ws *websocket.Conn, ch <-chan webrtc.ICECandidateInit) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case candidate, ok := <-ch:
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				return fmt.Errorf("error sending ice candidate: %w", err)
			}
			log.Println("sent candidate")
			if !ok {
				log.Println("ice gathering completed")
				return nil
			}
		}
	}
}

// addCandidates adds the recipient's ICE candidates from ch to the peer connection until
// there are no more or the context is cancelled.
func addCandidates(ctx context.Context, pc *webrtc.PeerConnection, ch <-chan webrtc.ICECandidateInit) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("ws caller ctx cancelled")
			return nil
		case candidate, ok := <-ch: // from the websocket
			if !ok {
				log.Println("no more recipient candidates")
				ch = nil
				continue
			}
			log.Println("recv recipient candidate")
			if err := pc.AddICECandidate(candidate); err != nil {
				return fmt.Errorf("error recieving ICE candidate: %w", err)
			}
		}
	}
}
