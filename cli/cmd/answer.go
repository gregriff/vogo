package cmd

import (
	"context"
	"fmt"
	"io"
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
		candidateChan = make(chan webrtc.ICECandidateInit, 10)
		connectedChan = make(chan struct{})
	)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		internal.OnICECandidate(c, candidateChan)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		internal.OnConnectionStateChange(s, connectedChan)
	})

	var answerWg sync.WaitGroup
	answerCtx, cancelAnswer := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cancelAnswer()
		answerWg.Wait()
	}()
	errorChan := make(chan error, 10)

	answerWg.Go(func() {
		defer cancelAnswer()
		defer func() {
			log.Println("answer goroutine COMPLETE")
		}()
		endpoint := fmt.Sprintf("/answer/%s", caller)
		ws, err := signaling.NewConnection(answerCtx, vogoServer, username, password, endpoint)
		if err != nil {
			errorChan <- fmt.Errorf("error creating websocket connection: %w", err)
			return
		}

		// wait to recv offer
		var offer webrtc.SessionDescription
		if err = websocket.JSON.Receive(ws, &offer); err != nil {
			if err == io.EOF {
				errorChan <- fmt.Errorf("Call not found") // could make this a sentinal
				return
			}
			errorChan <- fmt.Errorf("error reading offer from ws: %v", err)
			return
		}
		err = signaling.CreateAndSendAnswer(ws, pc, &offer, caller)
		if err != nil {
			errorChan <- fmt.Errorf("error creating or posting answer: %w", err)
			return
		}
		log.Println("answer sent")

		// send ice candidates to ws as they are gathered
		var (
			readWg           sync.WaitGroup
			callerCandidates = make(chan webrtc.ICECandidateInit)
		)
		defer signaling.CloseAndWait(ws, &readWg)
		readWg.Go(func() {
			signaling.ReadCandidates(ws, callerCandidates)
		})

		for {
			select {
			case <-answerCtx.Done():
				log.Println("ws answer ctx cancelled")
				return
			case iceCandidate, ok := <-candidateChan:
				if sErr := websocket.JSON.Send(ws, iceCandidate); sErr != nil {
					errorChan <- fmt.Errorf("error sending ice candidate: %w", sErr)
					return
				}
				log.Println("sent candidate")
				if !ok {
					log.Println("caller's ice gathering completed, channel closed")
					candidateChan = nil
					continue
				}
			// recv caller candidates from the websocket
			case callerCandidate, ok := <-callerCandidates:
				if !ok {
					callerCandidates = nil
					continue
				}
				log.Println("recv caller candidate")
				if iceErr := pc.AddICECandidate(callerCandidate); iceErr != nil {
					errorChan <- fmt.Errorf("error recieving ICE candidate: %w", iceErr)
					return
				}
			}
		}
	})

	var captureWaitGroup sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cancelCapture()
		captureWaitGroup.Wait()
	}()

	// setup microphone once call is connected and capture until cancelled
	captureWaitGroup.Go(func() {
		select {
		case <-captureCtx.Done():
			return
		case <-connectedChan:
			cancelAnswer()
			break
		}
		captureErr := audio.StartCapture(captureCtx, pc, track)
		if captureErr != nil {
			errorChan <- fmt.Errorf("error with capture device: %v", captureErr)
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
