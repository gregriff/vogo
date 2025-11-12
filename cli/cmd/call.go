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

	audioTrsv, tErr := pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	)
	if tErr != nil {
		fmt.Printf("error adding transceiver: %v", tErr)
		return
	}

	// setup microphone capture track
	captureTrack, _ := webrtc.NewTrackLocalStaticSample(
		configs.OpusCodecCapability,
		"captureTrack",
		"captureTrack"+username,
	)
	audioTrsv.Sender().ReplaceTrack(captureTrack)

	// var playbackWg sync.WaitGroup

	// playbackCtx, device, playbackErr := audio.SetupPlayback(pc, &playbackWg)
	// defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	// if playbackErr != nil {
	// 	fmt.Printf("error initializing playback system: %v", playbackErr)
	// 	return
	// }

	pc.OnICECandidate(internal.OnICECandidate)
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
	log.Println("waiting on gathering complete promise")
	<-webrtc.GatheringCompletePromise(pc)
	log.Println("waiting completed")

	// notifies microphone capture goroutine to begin capture
	callAnswered := make(chan struct{}, 1)

	// sending an error on this channel will abort the call process
	errorChan := make(chan error, 1)

	// TODO: could tie this to ICE candidate context? if calling needs to happen in parallel
	var callWg sync.WaitGroup
	callCtx, cancelCall := context.WithCancel(sigCtx)
	defer func() {
		cancelCall()
		callWg.Wait()
	}()

	callWg.Go(func() {
		defer cancelCall()
		sigClient := signaling.NewClient(vogoServer, username, password)
		recipientSd, callErr := signaling.CallFriend(callCtx, *sigClient, recipient, offer)
		if callErr != nil {
			errorChan <- fmt.Errorf("error while calling: %w", callErr)
			return
		}

		log.Println("RECIEVED ANSWER SD, adding remote SD")
		if sdpErr := pc.SetRemoteDescription(*recipientSd); sdpErr != nil {
			errorChan <- fmt.Errorf("error while setting remote description: %w", sdpErr)
			return
		}
		close(callAnswered)
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
		case <-callAnswered:
			captureErr := audio.StartCapture(captureCtx, pc, captureTrack)
			if captureErr != nil {
				errorChan <- fmt.Errorf("error with capture device: %w", captureErr)
			}
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
