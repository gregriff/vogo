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
	"github.com/gregriff/vogo/cli/internal/core"
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

	log.Println("creating recipient connection")
	pc, err := configs.NewPeerConnection(stunServer)
	if err != nil {
		fmt.Printf("error creating peer connection %v", err)
		return
	}
	defer configs.ClosePC(pc, true)

	// if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }

	track, err := configs.CreateAudioTrack(pc, username)
	if err != nil {
		log.Printf("error creating audio track: %v", err)
		return
	}

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

	log.Println("init playback device...")
	var playbackWg sync.WaitGroup
	playbackCtx, device, err := audio.SetupPlayback(pc, &playbackWg)
	defer audio.TeardownPlaybackResources(pc, playbackCtx, device, &playbackWg)
	if err != nil {
		fmt.Printf("error initializing playback system: %v", err)
		return
	}
	log.Println("playback device created")

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

		credentials := core.NewCredentials(vogoServer, username, password)
		err = core.AnswerAndConnect(answerCtx, pc, credentials, caller, candidates)
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
