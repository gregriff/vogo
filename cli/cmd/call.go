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
	pc, createErr := configs.NewPeerConnection(stunServer)
	if createErr != nil {
		fmt.Printf("error creating peer connection %v", createErr)
		return
	}
	defer func() {
		log.Println("forcing close of caller connection")
		if cErr := pc.Close(); cErr != nil {
			fmt.Printf("cannot forcefully close caller connection: %v\n", cErr)
		}
	}()

	track, tErr := configs.CreateAudioTrack(pc, username)
	if tErr != nil {
		log.Printf("error creating audio track: %v", tErr)
		return
	}

	// var playbackWg sync.WaitGroup

	// playbackCtx, device, playbackErr := audio.SetupPlayback(pc, &playbackWg)
	// defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	// if playbackErr != nil {
	// 	fmt.Printf("error initializing playback system: %v", playbackErr)
	// 	return
	// }

	candidateChan := make(chan webrtc.ICECandidateInit, 10)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		internal.OnICECandidate(c, candidateChan)
	})
	pc.OnConnectionStateChange(internal.OnConnectionStateChangeCaller) // TODO: remove

	// Create an offer to send to the other process
	log.Println("Creating offer")
	offer, oErr := pc.CreateOffer(nil)
	if oErr != nil {
		fmt.Printf("error creating offer: %v", oErr)
		return
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	log.Println("setting local description")
	if ldErr := pc.SetLocalDescription(offer); ldErr != nil {
		fmt.Printf("error setting local description: %v", ldErr)
		return
	}

	// Wait for all ICE candidates and include them all in the call request
	// TODO: don't use this and impl ICE trickle with vogo-server
	// log.Println("waiting on gathering complete promise")
	// <-webrtc.GatheringCompletePromise(pc)
	// log.Println("waiting completed")

	// notifies microphone capture goroutine to begin capture
	callAnswered := make(chan struct{}, 1)

	// sending an error on this channel will abort the call process
	errorChan := make(chan error, 10)

	// TODO: could tie this to ICE candidate context? if calling needs to happen in parallel
	var callWg sync.WaitGroup
	callCtx, cancelCall := context.WithCancel(sigCtx)
	defer func() {
		cancelCall()
		callWg.Wait()
	}()

	callWg.Go(func() {
		defer cancelCall()
		cfg, cfgErr := signaling.NewWsConfig(vogoServer, username, password, "/call")
		if cfgErr != nil {
			errorChan <- cfgErr
			return
		}

		ws, dialErr := cfg.DialContext(callCtx)
		if dialErr != nil {
			errorChan <- fmt.Errorf("error dialing ws: %w", dialErr)
			return
		}

		// post offer
		callReq := signaling.CallRequest{RecipientName: recipient, Sd: offer}
		log.Println("wrote offer to ws")
		if wErr := signaling.WriteWS(ws, callReq); wErr != nil {
			errorChan <- wErr
			return
		}

		// for websocket reading
		var (
			readErr    error
			readWg     sync.WaitGroup
			wsReadChan = make(chan webrtc.ICECandidateInit)

			// websocket messages
			candidate webrtc.ICECandidateInit
			answer    webrtc.SessionDescription
		)
		defer func() {
			ws.Close()
			readWg.Wait()
			log.Println("readWg closed")
		}()

		// TODO: here, we could send our ICE candidates to the ws while we wait for the answer

		readErr = websocket.JSON.Receive(ws, &answer)
		if readErr != nil {
			errorChan <- fmt.Errorf("error reading answer from ws: %v", readErr)
			ws.Close()
			return
		}
		if answer.SDP == "" {
			log.Println("empty answer")
			return
		}
		if sdErr := pc.SetRemoteDescription(answer); sdErr != nil {
			errorChan <- fmt.Errorf("error while setting remote description: %w", sdErr)
			return
		}
		close(callAnswered)

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
			case <-callCtx.Done():
				log.Println("ws answer connection cancelled")
				return
			case iceCandidate := <-candidateChan:
				if wErr := signaling.WriteWS(ws, iceCandidate); wErr != nil {
					errorChan <- wErr
					return
				}
			case answerCandidate := <-wsReadChan:
				if iceErr := pc.AddICECandidate(answerCandidate); iceErr != nil {
					errorChan <- fmt.Errorf("error recieving ICE candidate: %w", iceErr)
					return
				}
			}
		}
	})

	var captureWg sync.WaitGroup
	captureCtx, cancelCapture := context.WithCancel(sigCtx)
	defer func() {
		cancelCapture()
		captureWg.Wait()
	}()

	// setup microphone once call is answered and capture until cancelled
	captureWg.Go(func() {
		select {
		case <-captureCtx.Done(): // if call fails
			return
		case <-callAnswered: // TODO: this should actually wait on the onICEConnected? event
			break
		}
		captureErr := audio.StartCapture(captureCtx, pc, track)
		if captureErr != nil {
			errorChan <- fmt.Errorf("error with capture device: %w", captureErr)
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
