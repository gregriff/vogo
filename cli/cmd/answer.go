package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
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

	// TODO:
	// - define audio information

	// var candidatesMux sync.Mutex
	// pendingCandidates := make([]*webrtc.ICECandidate, 0)

	api := configs.NewWebRTC()
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{stunServer},
			},
		},
	}

	log.Println("creating answerer connection")
	pc, createErr := api.NewPeerConnection(config)
	if createErr != nil {
		panic(createErr)
	}
	defer func() {
		log.Println("closing answerer connection")
		if cErr := pc.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	// if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }

	var (
		audioTrsv *webrtc.RTPTransceiver
		tErr      error
	)
	if audioTrsv, tErr = pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	); tErr != nil {
		panic(tErr)
	}

	// setup microphone capture track
	captureTrack, _ := webrtc.NewTrackLocalStaticSample(
		configs.OpusCodecCapability,
		"captureTrack",
		"captureTrack"+username,
	)
	audioTrsv.Sender().ReplaceTrack(captureTrack)

	playbackCtx, device := audio.SetupPlayback(pc)
	defer playbackCtx.Uninit()
	defer playbackCtx.Free()
	defer device.Uninit()

	pc.OnICECandidate(internal.OnICECandidate)

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

	log.Println("getting caller SD from vogo server, caller Name: ", caller)
	sigClient := signaling.NewClient(vogoServer, username, password)

	callerSd := signaling.GetCallerSd(*sigClient, caller)

	log.Println("setting caller's remote description")
	if err := pc.SetRemoteDescription(*callerSd); err != nil {
		log.Fatalf("error setting callers remote description: %v", err)
	}

	log.Println("answerer creating answer")
	answer, aErr := pc.CreateAnswer(nil)
	if aErr != nil {
		log.Fatalf("error creating answer: %v", aErr)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	log.Println("answerer setting localDescription and listening for UDP")
	aErr = pc.SetLocalDescription(answer)
	if aErr != nil {
		log.Fatalf("error setting local description: %v", aErr)
	}

	// Create a channel to wait for gathering and Wait for gathering to finish
	// TODO: don't use this and impl ICE trickle with vogo-server
	log.Println("waiting on gathering complete promise")
	<-webrtc.GatheringCompletePromise(pc)
	log.Println("waiting completed")

	localDescription := *pc.LocalDescription()
	signaling.PostAnswer(*sigClient, caller, localDescription)

	// TODO: the above should run in a goroutine with a context and
	// signal to the below that the ansewr has been completed.

	// // setup microphone capture track
	// captureTrack, _ := webrtc.NewTrackLocalStaticSample(
	// 	configs.OpusCodecCapability,
	// 	uuid.NewString(),
	// 	uuid.NewString(),
	// )
	// audioTrsv.Sender().ReplaceTrack(captureTrack)

	captureCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setup mic and capture indefinitely
	go func() {
		audio.StartCapture(captureCtx, pc, captureTrack)
	}()

	// Block forever
	log.Println("Answer complete, mic initalized, blocking until ctrl C")

	// TODO: tie this to the context of the peer connection
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// block until ctrl C
	<-sigChan

	// all contexts defined above should now have their cancel funcs run
}
