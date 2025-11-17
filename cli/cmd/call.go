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

	var (
		candidateChan = make(chan webrtc.ICECandidateInit, 10)
		connectedChan = make(chan struct{})
	)
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		internal.OnICECandidate(c, candidateChan)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		internal.OnConnectionStateChangeCaller(s, connectedChan)
	})

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

	// sending an error on this channel will abort the call process
	errorChan := make(chan error, 10)

	var callWg sync.WaitGroup
	callCtx, cancelCall := context.WithCancel(sigCtx)
	defer func() {
		cancelCall()
		log.Println("cancelCall called, waiting...")
		callWg.Wait()
		log.Println("call wg completed")
	}()

	callWg.Go(func() {
		defer cancelCall()
		ws, wsErr := signaling.NewWebsocketConn(callCtx, vogoServer, username, password, "/call")
		if wsErr != nil {
			errorChan <- fmt.Errorf("error creating websocket connection: %w", wsErr)
			return
		}

		// send offer
		callReq := signaling.CallRequest{RecipientName: recipient, Sd: offer}
		log.Println("wrote offer to ws")
		if sErr := websocket.JSON.Send(ws, callReq); sErr != nil {
			errorChan <- sErr
			return
		}

		var sendIceWg sync.WaitGroup
		sendIceCtx, cancelSendIce := context.WithCancel(callCtx)
		defer func() {
			cancelSendIce()
			sendIceWg.Wait()
		}()

		// gather local ice candidates and write to websocket
		sendIceWg.Go(func() {
			defer cancelSendIce()
			defer func() {
				log.Println("sendIce goroutine finished")
			}()
			for {
				select {
				case <-sendIceCtx.Done():
					return
				case iceCandidate, ok := <-candidateChan:
					if sErr := websocket.JSON.Send(ws, iceCandidate); sErr != nil {
						errorChan <- fmt.Errorf("error sending ice candidate: %w", sErr)
						return
					}
					log.Println("sent candidate")
					if !ok {
						log.Println("caller's ice gathering completed, channel closed")
						return
					}
				}
			}
		})

		// wait to recv answer
		var answer webrtc.SessionDescription
		readErr := websocket.JSON.Receive(ws, &answer)
		if readErr != nil {
			errorChan <- fmt.Errorf("error reading answer from ws: %v", readErr)
			return
		}
		if sdErr := pc.SetRemoteDescription(answer); sdErr != nil {
			errorChan <- fmt.Errorf("error while setting remote description: %w", sdErr)
			return
		}
		log.Println("recieved answer")

		var (
			readWg   sync.WaitGroup
			readChan = make(chan webrtc.ICECandidateInit)
		)
		defer func() {
			// ws.Close will unblock any reads on the connection
			if cErr := ws.Close(); cErr != nil {
				log.Printf("error closing ws in defer: %v", cErr)
			}
			readWg.Wait()
		}()
		readWg.Go(func() {
			signaling.ReadCandidates(ws, readChan)
		})

		for {
			select {
			case <-callCtx.Done():
				log.Println("ws caller ctx cancelled")
				return
			// recv answerer candidates from the websocket
			case answerCandidate, ok := <-readChan:
				// all answerer candidates have been gathered, now we just
				// wait on the callCtx to cancel
				if !ok {
					readChan = nil
					continue
				}
				log.Println("recv answerer candidate")
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

	// setup microphone once call is connected and capture until cancelled
	captureWg.Go(func() {
		select {
		case <-captureCtx.Done(): // if call fails
			return
		case <-connectedChan:
			cancelCall()
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
