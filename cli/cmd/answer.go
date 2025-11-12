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

	// var candidatesMux sync.Mutex
	// pendingCandidates := make([]*webrtc.ICECandidate, 0)

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
	captureTrack, trackErr := webrtc.NewTrackLocalStaticSample(
		configs.OpusCodecCapability,
		"captureTrack",
		"captureTrack"+username,
	)
	if trackErr != nil {
		fmt.Printf("error initalizing capture track: %v", trackErr)
		return
	}
	audioTrsv.Sender().ReplaceTrack(captureTrack)

	var playbackWg sync.WaitGroup

	playbackCtx, device, playbackErr := audio.SetupPlayback(pc, &playbackWg)
	defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	if playbackErr != nil {
		fmt.Printf("error initializing playback system: %v", playbackErr)
		return
	}

	pc.OnICECandidate(internal.OnICECandidate)
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

	log.Println("getting caller SD from vogo server, caller Name: ", caller)
	sigClient := signaling.NewClient(vogoServer, username, password)

	callerSd, sdErr := signaling.GetCallerSd(*sigClient, caller)
	if sdErr != nil {
		fmt.Printf("error getting callers session description: %v", sdErr)
		return
	}

	log.Println("setting caller's remote description")
	if sdErr = pc.SetRemoteDescription(*callerSd); sdErr != nil {
		fmt.Printf("error setting callers remote description: %v", sdErr)
		return
	}

	log.Println("answerer creating answer")
	answer, aErr := pc.CreateAnswer(nil)
	if aErr != nil {
		fmt.Printf("error creating answer: %v", aErr)
		return
	}

	// Sets the LocalDescription, and starts our UDP listeners
	log.Println("answerer setting localDescription and listening for UDP")
	ldErr := pc.SetLocalDescription(answer)
	if ldErr != nil {
		fmt.Printf("error setting local description: %v", ldErr)
		return
	}

	// Create a channel to wait for gathering and Wait for gathering to finish
	// TODO: don't use this and impl ICE trickle with vogo-server
	log.Println("waiting on gathering complete promise")
	<-webrtc.GatheringCompletePromise(pc)
	log.Println("waiting completed")

	answerErr := signaling.PostAnswer(*sigClient, caller, *pc.LocalDescription())
	if answerErr != nil {
		fmt.Printf("error while posting answer: %v", answerErr)
		return
	}

	// TODO: the above should run in a goroutine with a context and
	// signal to the below that the ansewr has been completed.

	var captureWaitGroup sync.WaitGroup
	captureCtx, cCtxCancel := context.WithCancel(sigCtx)
	defer func() { // wait for capture device teardown
		cCtxCancel()
		captureWaitGroup.Wait()
	}()
	errorChan := make(chan error, 1)

	// setup mic and capture until the above cancel func is run
	captureWaitGroup.Go(func() {
		captureErr := audio.StartCapture(captureCtx, pc, captureTrack)
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
