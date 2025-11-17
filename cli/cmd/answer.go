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

var answerCmd = &cobra.Command{
	Use:   "answer",
	Short: "Answer a call from a friend",
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
			return fmt.Errorf("caller must be specified as an argument")
		}

		caller := args[0]
		if len(caller) > 16 {
			return fmt.Errorf("caller string too long")
		}
		viper.Set("caller", caller)
		return nil
	},
	Run: answerCall,
}

func init() {
	rootCmd.AddCommand(answerCmd)
}

func answerCall(_ *cobra.Command, _ []string) {
	_, username, password, vogoServer, stunServer, caller := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("servers.vogo-origin"),
		viper.GetString("servers.stun-origin"),
		viper.GetString("caller")

	// parent context for all other contexts to be derived from
	sigCtx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("creating recipient connection")
	pc, err := configs.NewPeerConnection(stunServer)
	if err != nil {
		fmt.Printf("error creating peer connection %v", err)
		return
	}
	defer func() {
		fmt.Println("forcing close of recipient connection")
		if err := pc.Close(); err != nil {
			fmt.Printf("cannot forcefully close recipient connection: %v\n", err)
		}
	}()

	// if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }

	track, err := configs.CreateAudioTrack(pc, username)
	if err != nil {
		log.Printf("error creating audio track: %v", err)
		return
	}

	log.Println("init playback device...")
	var playbackWg sync.WaitGroup
	playbackCtx, device, err := audio.SetupPlayback(pc, &playbackWg)
	defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	if err != nil {
		fmt.Printf("error initializing playback system: %v", err)
		return
	}
	log.Println("playback device created")

	var (
		candidates = make(chan webrtc.ICECandidateInit, 10)
		connected  = make(chan struct{})
	)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		internal.OnICECandidate(c, candidates)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		internal.OnConnectionStateChange(s, connected)
	})

	var answerWg sync.WaitGroup
	answerCtx, cancelAnswer := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cancelAnswer()
		answerWg.Wait()
		log.Println("answer wg completed")
	}()
	errorChan := make(chan error, 10)

	answerWg.Go(func() {
		defer cancelAnswer()

		credentials := signaling.NewCredentials(vogoServer, username, password)
		err = answerAndConnect(answerCtx, pc, credentials, caller, candidates)
		if err != nil {
			errorChan <- err
			return
		}
	})

	var captureWg sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cancelCapture()
		captureWg.Wait()
	}()

	// setup microphone once call is connected and capture until cancelled
	captureWg.Go(func() {
		select {
		case <-captureCtx.Done():
			return
		case <-connected:
			cancelAnswer()
			break
		}
		if err = audio.StartCapture(captureCtx, pc, track); err != nil {
			errorChan <- fmt.Errorf("error with capture device: %v", err)
		}
	})

	// block until ctrl C or an error in capture goroutine
	select {
	case err := <-errorChan:
		fmt.Println(err)
		break
	case <-sigCtx.Done():
		break
	}
}

// answerAndConnect answers and establishes a voice call with a friend client. It
// uses a websocket connection to a vogo server to handle signaling and connecting.
// It uses trickle-ICE for fast connection.
func answerAndConnect(
	ctx context.Context,
	pc *webrtc.PeerConnection,
	credentials *signaling.Credentials,
	caller string,
	candidates <-chan webrtc.ICECandidateInit,
) error {
	endpoint := fmt.Sprintf("/answer/%s", caller)
	ws, err := signaling.NewConnection(ctx, credentials, endpoint)
	if err != nil {
		return fmt.Errorf("error creating websocket: %w", err)
	}

	offer, err := signaling.RecieveOffer(ws)
	if err != nil {
		return fmt.Errorf("error recieving offer: %w", err)
	}
	err = signaling.CreateAndSendAnswer(ws, pc, offer, caller)
	if err != nil {
		return fmt.Errorf("error creating or posting answer %w", err)
	}
	log.Println("answer sent")

	// read caller candidates from ws as they come in
	var (
		readWg           sync.WaitGroup
		callerCandidates = make(chan webrtc.ICECandidateInit)
	)
	defer signaling.CloseAndWait(ws, &readWg)

	readWg.Go(func() {
		signaling.ReadCandidates(ws, callerCandidates)
	})
	err = exchangeCandidates(ctx, ws, pc, candidates, callerCandidates)
	if err != nil {
		return err
	}
	return nil
}

// exchangeCandidates handles sending the client's ICE candidates to the websocket to be
// forwarded to the caller, and adding the caller's candidates recieved from the websocket.
// It does this until both sources are exhausted, or until the context is cancelled.
func exchangeCandidates(
	ctx context.Context,
	ws *websocket.Conn,
	pc *webrtc.PeerConnection,
	candidates, callerCandidates <-chan webrtc.ICECandidateInit,
) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("ws answer ctx cancelled")
			return nil
		case candidate, ok := <-candidates:
			if err := websocket.JSON.Send(ws, candidate); err != nil {
				return fmt.Errorf("error sending ice candidate: %w", err)
			}
			log.Println("sent candidate")
			if !ok {
				log.Println("gathering completed")
				candidates = nil
				continue
			}
		// recv caller candidates from the websocket
		case callerCandidate, ok := <-callerCandidates:
			if !ok {
				log.Println("no more caller candidates")
				callerCandidates = nil
				continue
			}
			log.Println("recv caller candidate")
			if err := pc.AddICECandidate(callerCandidate); err != nil {
				return fmt.Errorf("error recieving ICE candidate: %w", err)
			}
		}
	}
}
