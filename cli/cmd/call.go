package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gen2brain/malgo"
	"github.com/gregriff/vogo/cli/configs"
	"github.com/gregriff/vogo/cli/internal"
	"github.com/gregriff/vogo/cli/internal/services/signaling"
	"github.com/pion/opus"
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

	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	// configure playback device
	ctx, _ := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	defer ctx.Uninit()

	const numChannels = 2
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = numChannels
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	// Buffer for decoded audio
	playbackBuffer := internal.NewRingBuffer(2_000_000)

	// read into output sample, for output to speaker device
	onSendFrames := func(pOutputSample, _ []byte, framecount uint32) {
		playbackBuffer.Read(pOutputSample)
	}

	// init playback device
	device, deviceErr := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	if deviceErr != nil {
		panic(deviceErr)
	}
	device.Start()
	defer device.Uninit()

	decoder := opus.NewDecoder()
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// opusPacket := &codecs.OpusPacket{}
		pcmBuffer := make([]byte, 23040)

		for { // Read RTP packets
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					break // Track closed, exit loop
				}
				continue // Temporary error, keep trying
			}

			// Decode Opus to PCM
			bandwidth, _, decodeErr := decoder.Decode(packet.Payload, pcmBuffer)
			if decodeErr != nil {
				continue
			}

			// Calculate actual bytes decoded, assume 20ms frame (most common)
			samplesPerChannel := bandwidth.SampleRate() * 20 / 960 // 960 for 48kHz
			bytesDecoded := samplesPerChannel * numChannels * 2    // *2 for S16LE

			// TODO: probably should calculate bytes decoded better. but keep in mind WEBRTC could change
			// sample rate so continue to use that.
			// deviceConfig := malgo.DefaultDeviceConfig(malgo.Duplex)
			// deviceConfig.Capture.Format = malgo.FormatS16
			// deviceConfig.Capture.Channels = 1
			// deviceConfig.Playback.Format = malgo.FormatS16
			// deviceConfig.Playback.Channels = 1
			// deviceConfig.SampleRate = 44100
			// deviceConfig.Alsa.NoMMap = 1

			// var playbackSampleCount uint32
			// var capturedSampleCount uint32
			// pCapturedSamples := make([]byte, 0)

			// sizeInBytes := uint32(malgo.SampleSizeInBytes(deviceConfig.Capture.Format))
			// onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {

			// 	sampleCount := framecount * deviceConfig.Capture.Channels * sizeInBytes

			// 	newCapturedSampleCount := capturedSampleCount + sampleCount

			// 	pCapturedSamples = append(pCapturedSamples, pSample...)

			// 	capturedSampleCount = newCapturedSampleCount

			// }

			// Write decoded PCM to ring buffer
			// Malgo will pull from this buffer to play
			playbackBuffer.Write(pcmBuffer[:bytesDecoded])
		}
	})

	pc.OnICECandidate(internal.OnICECandidate)

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	pc.OnConnectionStateChange(internal.OnConnectionStateChange)

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

		log.Println("RECIEVED ANSWER SD, adding remote SD")
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
