package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
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

	log.Println("creating answerer connection")
	pc, createErr := configs.NewPeerConnection(stunServer)
	if createErr != nil {
		fmt.Printf("error creating peer connection %v", createErr)
		return
	}
	defer func() {
		fmt.Println("forcing close of answerer connection")
		if cErr := pc.Close(); cErr != nil {
			fmt.Printf("cannot forcefully close answerer connection: %v\n", cErr)
		}
	}()

	// if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }

	track, tErr := configs.CreateAudioTrack(pc, username)
	if tErr != nil {
		log.Printf("error creating audio track: %v", tErr)
		return
	}

	var playbackWg sync.WaitGroup

	playbackCtx, device, playbackErr := audio.SetupPlayback(pc, &playbackWg)
	defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	if playbackErr != nil {
		fmt.Printf("error initializing playback system: %v", playbackErr)
		return
	}

	candidateChan := make(chan webrtc.ICECandidateInit, 15)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		internal.OnICECandidate(c, candidateChan)
	})
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

	var answerWg sync.WaitGroup
	answerCtx, cancelAnswer := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cancelAnswer()
		answerWg.Wait()
	}()
	errorChan := make(chan error, 10)

	// notifies microphone capture goroutine to begin capture
	answerSent := make(chan struct{})

	answerWg.Go(func() {
		defer cancelAnswer()
		endpoint := fmt.Sprintf("/answer/%s", caller)
		cfg, cfgErr := signaling.NewWsConfig(vogoServer, username, password, endpoint)
		if cfgErr != nil {
			errorChan <- cfgErr
			return
		}

		ws, dialErr := cfg.DialContext(answerCtx)
		if dialErr != nil {
			errorChan <- fmt.Errorf("error dialing ws: %w", dialErr)
			return
		}

		// for websocket reading
		var (
			readErr    error
			readWg     sync.WaitGroup
			wsReadChan = make(chan webrtc.ICECandidateInit)

			// websocket messages
			candidate webrtc.ICECandidateInit
			offer     webrtc.SessionDescription
		)
		defer func() {
			ws.Close()
			readWg.Wait()
			log.Println("readWg closed")
		}()

		readErr = websocket.JSON.Receive(ws, &offer)
		if readErr != nil {
			log.Printf("error reading offer from ws: %v", readErr)
			ws.WriteClose(http.StatusBadRequest)
			return
		}
		if offer.SDP == "" {
			log.Println("empty offer")
			ws.WriteClose(http.StatusBadRequest)
			return
		}
		aErr := signaling.CreateAndPostAnswer(ws, pc, &offer, caller)
		if aErr != nil {
			errorChan <- fmt.Errorf("error creating or posting answer: %w", aErr)
			return
		}
		log.Println("answer sent")
		close(answerSent)

		readWg.Go(func() {
			for {
				readErr = websocket.JSON.Receive(ws, &candidate)
				if readErr != nil {
					if readErr == io.EOF {
						ws.Close()
						return
					}
					log.Printf("error reading from ws: %v", readErr)
					ws.Close()
					return
				}
				wsReadChan <- candidate
			}
		})

		for {
			select {
			case <-answerCtx.Done():
				log.Println("ws answer connection cancelled")
				return
			case iceCandidate := <-candidateChan:
				if wErr := signaling.WriteWS(ws, iceCandidate); wErr != nil {
					errorChan <- wErr
					return
				}
			case callerCandidate := <-wsReadChan:
				if callerCandidate.Candidate == "" {
					log.Println("empty candidate recieved")
					return
				}
				if iceErr := pc.AddICECandidate(callerCandidate); iceErr != nil {
					errorChan <- fmt.Errorf("error recieving ICE candidate: %w", iceErr)
					return
				}
			}
		}
	})

	// TODO: the above should run in a goroutine with a context and
	// signal to the below that the ansewr has been completed.

	var captureWaitGroup sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cancelCapture()
		captureWaitGroup.Wait()
	}()

	// setup mic and capture until the above cancel func is run
	captureWaitGroup.Go(func() {
		// TODO: need to wait for onICEconnected
		select {
		case <-captureCtx.Done():
			return
		case <-answerSent:
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
