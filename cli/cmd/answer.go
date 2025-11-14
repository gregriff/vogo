package cmd

import (
	"context"
	"encoding/json"
	"errors"
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

	candidateChan := make(chan webrtc.ICECandidateInit, 10)
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
	errorChan := make(chan error, 1)

	// notifies microphone capture goroutine to begin capture
	answerSent := make(chan struct{}, 1)

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
			buf        = make([]byte, 1500)
			n          int // bytes read per message
			readErr    error
			mu         sync.Mutex
			readWg     sync.WaitGroup
			wsReadChan = make(chan struct{})

			// websocket messages
			candidate webrtc.ICECandidateInit
			offer     webrtc.SessionDescription
		)
		defer func() {
			ws.Close()
			readWg.Wait()
			log.Println("readWg closed")
		}()

		readWg.Go(func() {
			for {
				mu.Lock()
				// TODO: handle when this unblocks due to ws closing
				n, readErr = ws.Read(buf)
				mu.Unlock()
				if readErr != nil {
					errorChan <- fmt.Errorf("error reading from ws: %w", readErr)
					return
				}
				wsReadChan <- struct{}{}
			}
		})

		for {
			select {
			case <-answerCtx.Done():
				log.Println("ws answer connection cancelled")
				return
			case candidate := <-candidateChan:
				if wErr := signaling.WriteWS(ws, candidate); wErr != nil {
					errorChan <- wErr
					return
				}
			case <-wsReadChan:
				mu.Lock()
				switch {
				case json.Unmarshal(buf[:n], &offer) == nil && offer.SDP != "":
					aErr := signaling.CreateAndPostAnswer(ws, pc, &offer, caller)
					if aErr != nil {
						errorChan <- fmt.Errorf("error creating or posting answer: %w", aErr)
						mu.Unlock()
						return
					}
					close(answerSent)
				case json.Unmarshal(buf[:n], &candidate) == nil && candidate.Candidate != "":
					if iceErr := pc.AddICECandidate(candidate); iceErr != nil {
						errorChan <- fmt.Errorf("error recieving ICE candidate: %w", iceErr)
						mu.Unlock()
						return
					}
				default:
					errorChan <- errors.New("unknown message")
					mu.Unlock()
					return
				}
				mu.Unlock()
			}
		}
	})

	// log.Println("getting caller SD from vogo server, caller Name: ", caller)
	// sigClient := signaling.NewClient(vogoServer, username, password)

	// callerSd, sdErr := signaling.GetCallerSd(*sigClient, caller)
	// if sdErr != nil {
	// 	fmt.Printf("error getting callers session description: %v", sdErr)
	// 	return
	// }

	// log.Println("setting caller's remote description")
	// if sdErr = pc.SetRemoteDescription(*callerSd); sdErr != nil {
	// 	fmt.Printf("error setting callers remote description: %v", sdErr)
	// 	return
	// }

	// log.Println("answerer creating answer")
	// answer, aErr := pc.CreateAnswer(nil)
	// if aErr != nil {
	// 	fmt.Printf("error creating answer: %v", aErr)
	// 	return
	// }

	// // Sets the LocalDescription, and starts our UDP listeners
	// log.Println("answerer setting localDescription and listening for UDP")
	// ldErr := pc.SetLocalDescription(answer)
	// if ldErr != nil {
	// 	fmt.Printf("error setting local description: %v", ldErr)
	// 	return
	// }

	// TODO: once ws is working, do we even need to check to see if gathering is completed?
	// prob not
	// log.Println("waiting on gathering complete promise")
	// <-webrtc.GatheringCompletePromise(pc)
	// log.Println("waiting completed")

	// TODO: this should send on ws immediately
	// answerErr := signaling.PostAnswer(*sigClient, caller, *pc.LocalDescription())
	// if answerErr != nil {
	// 	fmt.Printf("error while posting answer: %v", answerErr)
	// 	return
	// }

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
		<-answerSent
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
