package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gregriff/vogo/cli/internal"
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
		username, password := viper.GetString("username"), viper.GetString("password")
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
	_, username, password, vogoServer, caller := viper.GetBool("debug"),
		viper.GetString("username"),
		viper.GetString("password"),
		viper.GetString("vogo-server"),
		viper.GetString("caller")

	// TODO:
	// - define audio information

	// var candidatesMux sync.Mutex
	// pendingCandidates := make([]*webrtc.ICECandidate, 0)

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	log.Println("creating answerer connection")
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		log.Println("closing answerer connection")
		if cErr := pc.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	pc.OnICECandidate(internal.OnICECandidate)

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

	/////////////////////////////////////////////////////////////////////////////
	// Register data channel creation handling
	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("Answerer: New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())

		// Register channel opening handling
		dataChannel.OnOpen(func() {
			fmt.Printf(
				"Answerer: Data channel '%s'-'%d' open. Messages will now be sent to any connected DataChannels every 5 seconds\n",
				dataChannel.Label(), dataChannel.ID(),
			)

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				message := "pong"

				// Send the message as text
				fmt.Printf("Answerer Sending '%s'\n", message)
				if sendTextErr := dataChannel.SendText(message); sendTextErr != nil {
					panic(sendTextErr)
				}
			}
		})

		// Register text message handling
		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Answerer recieved Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
		})
	})

	// answer(caller, vogoServer, pc)

	log.Println("getting caller SD from vogo server, caller Name: ", caller)
	sigClient := signaling.NewClient(vogoServer, username, password)

	callerSd := signaling.GetCallerSd(*sigClient, caller)

	log.Println("setting caller's remote description")
	if err := pc.SetRemoteDescription(*callerSd); err != nil {
		log.Fatalf("error setting callers remote description: %v", err)
	}

	log.Println("answerer creating answer")
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Fatalf("error creating answer: %v", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	log.Println("answerer setting localDescription and listening for UDP")
	err = pc.SetLocalDescription(answer)
	if err != nil {
		log.Fatalf("error setting local description: %v", err)
	}

	// Create a channel to wait for gathering and Wait for gathering to finish
	// TODO: don't use this and impl ICE trickle with vogo-server
	log.Println("waiting on gathering complete promise")
	<-webrtc.GatheringCompletePromise(pc)
	log.Println("waiting completed")

	localDescription := *pc.LocalDescription()
	signaling.PostAnswer(*sigClient, caller, localDescription)

	/////////

	// Block forever
	log.Println("Answer complete, blocking until ctrl C")

	// TODO: tie this to the context of the peer connection
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// block until ctrl C
	<-sigChan
}
