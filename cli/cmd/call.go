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

var callCmd = &cobra.Command{
	Use:   "call",
	Short: "Call a friend",
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
	_, vogoServer, recipient, username, password := viper.GetBool("debug"),
		viper.GetString("vogo-server"),
		viper.GetString("recipient"),
		viper.GetString("username"),
		viper.GetString("password")

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	log.Println("creating peer connection")
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		log.Println("closing peer connection")
		if cErr := pc.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	pc.OnICECandidate(internal.OnICECandidate)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Create a datachannel with label 'data'
	dataChannel, err := pc.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}
	///////////////////////////////////////////////////////////////////////////////////////////////////

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf(
			"Data channel '%s'-'%d' open. Messages will now be sent to any connected DataChannels every 5 seconds\n",
			dataChannel.Label(), dataChannel.ID(),
		)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			message := "ping"

			// Send the message as text
			fmt.Printf("Sending '%s'\n", message)
			if sendTextErr := dataChannel.SendText(message); sendTextErr != nil {
				panic(sendTextErr)
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
	})
	///////////////////////////////////////////////////////////////////////////////////////////////////

	// Create an offer to send to the other process
	log.Println("Creating offer")
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	log.Println("setting local description")
	if err = pc.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Wait for all ICE candidates and include them all in the call request
	// TODO: don't use this and impl ICE trickle with vogo-server
	log.Println("waiting on gathering complete promise")
	<-webrtc.GatheringCompletePromise(pc)
	log.Println("waiting completed")

	go func() {
		sigClient := signaling.NewClient(vogoServer, username, password)
		recipientSd, callErr := signaling.CallFriend(*sigClient, recipient, offer)
		if callErr != nil {
			panic(fmt.Errorf("call error: %w", callErr))
		}

		log.Println("RECIEVED ANSWER SD, adding remote SD:", recipientSd)
		if sdpErr := pc.SetRemoteDescription(*recipientSd); sdpErr != nil {
			panic(sdpErr)
		}
	}()

	// Block forever
	log.Println("Sent offer, blocking until ctrl C")

	// TODO: tie this to the context of the offer request
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// block until ctrl C
	<-sigChan
	// NOTE: AudioRecieverStats{} implements a jitterbuffer...
}
