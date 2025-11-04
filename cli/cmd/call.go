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

	api := configs.NewWebRTC()
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{stunServer},
			},
		},
	}

	log.Println("creating peer connection")
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		log.Println("closing peer connection")
		if cErr := pc.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

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

	playbackCtx, device := audio.SetupPlayback(pc)
	defer playbackCtx.Uninit()
	defer playbackCtx.Free()
	defer device.Uninit()

	pc.OnICECandidate(internal.OnICECandidate)

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

	// Create an offer to send to the other process
	log.Println("Creating offer")
	offer, oErr := pc.CreateOffer(nil)
	if oErr != nil {
		panic(oErr)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	log.Println("setting local description")
	if oErr = pc.SetLocalDescription(offer); oErr != nil {
		panic(oErr)
	}

	// Wait for all ICE candidates and include them all in the call request
	// TODO: don't use this and impl ICE trickle with vogo-server
	log.Println("waiting on gathering complete promise")
	<-webrtc.GatheringCompletePromise(pc)
	log.Println("waiting completed")

	captureCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// handle signalling and on success init microphone capture
	go func() {
		sigClient := signaling.NewClient(vogoServer, username, password)
		recipientSd, callErr := signaling.CallFriend(*sigClient, recipient, offer)
		if callErr != nil {
			panic(fmt.Errorf("call error: %w", callErr))
		}

		log.Println("RECIEVED ANSWER SD, adding remote SD")
		if sdpErr := pc.SetRemoteDescription(*recipientSd); sdpErr != nil {
			panic(sdpErr)
		}

		// TODO: the above should run in a seperate goroutine as the stuff below and should
		// signal to the below that the ansewr has been recieved.

		// setup microphone capture track
		captureTrack, _ := webrtc.NewTrackLocalStaticSample(
			configs.OpusCodecCapability,
			"captureTrack",
			"captureTrack"+username,
		)
		audioTrsv.Sender().ReplaceTrack(captureTrack)

		// setup microphone and capture until cancelled
		audio.StartCapture(captureCtx, pc, captureTrack)
	}()

	// Block forever
	log.Println("Sent offer, blocking until ctrl C")

	// TODO: tie this to the context of the offer request
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// block until ctrl C
	<-sigChan

	// all contexts defined above should now have their cancel funcs run
	// NOTE: AudioRecieverStats{} implements a jitterbuffer...
}
