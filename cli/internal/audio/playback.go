package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

// SetupPlayback initializes the playback device with malgo, and defines the callback that is run per remote-track, that
// reads the audio from the network and places it in the buffer for the playback device to read from. This function
// is used by voice calls (1:1)
// TODO: should this return the pcm stuff so that onTrack can be called by other goroutines once PCs are created?
func SetupPlayback(pc *webrtc.PeerConnection, wg *sync.WaitGroup) (
	deviceCtx *malgo.AllocatedContext,
	device *malgo.Device,
	err error,
) {
	deviceCtx, device, pcm, err := createCallDevice()
	if err != nil {
		err = fmt.Errorf("error initalizing playback device: %w", err)
		return
	}

	decoder, err := opus.NewDecoder(SampleRate, NumChannels)
	if err != nil {
		err = fmt.Errorf("decoder init error: %w", err)
		return
	}

	// this func runs for every remote track connected to this peer connection
	// this is where the decoder writes pcm from the network
	// note: this callback should not panic
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		wg.Add(1)
		defer wg.Done()

		decodeBuf := make([]int16, pcmBufferSize)
		for {
			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					return // Track closed, exit loop
				}
				log.Println("PACKET READ ERR: ", readErr)
				continue // Temporary error, keep trying
			}

			// TODO: check for 0 samples decoded and call PLC?
			samplesDecoded, decodeErr := decoder.Decode(packet.Payload, decodeBuf)
			if decodeErr != nil {
				log.Println("DECODE ERROR: ", decodeErr.Error())
				continue
			}

			framesDecoded := samplesDecoded * NumChannels
			// Write decoded PCM to playback buffer, which malgo will pull from for playback
			pcm.mu.Lock()
			pcm.buf = append(pcm.buf, decodeBuf[:framesDecoded]...)
			pcm.mu.Unlock()
		}
	})
	return
}

// SetupPlaybackChannel initializes the playback device with malgo,
// using the pcm struct for handling multiple audio streams.
// // TODO: combine ctx and device into a Speaker struct
func SetupPlaybackChannel(pcmBuffers *ChannelStreams) (
	deviceCtx *malgo.AllocatedContext,
	device *malgo.Device,
	err error,
) {
	deviceCtx, device, err = createChannelDevice(pcmBuffers)
	if err != nil {
		err = fmt.Errorf("error initalizing playback device: %w", err)
	}
	return
}

// OnRemoteTrack handles decoding decoding opus audio for each remote track of a PeerConnection. It
// should be attached to each PeerConnection using pc.OnTrack(). For vogo, each PC should only have one
// RemoteTrack. Decoded audio is written to pcmBuffers, from which the speaker goroutine reads and mixes
// with other PC's audio streams for playback. This should only be used for multi-user voice chat channels/rooms.
// NOTE: DecodeFEC and DecodePLC are available for later use
// NOTE: if text remote tracks are added, this will have to not add those to audio stream struct
func OnRemoteTrack(
	wg *sync.WaitGroup,
	pcmBuffers *ChannelStreams,
) func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// this func runs for every remote track connected to this peer connection
	// this is where the decoder writes pcm from the network
	// note: realize that this code will run multiple times if more than one remote track is connected (multi-user voice chat)
	// note: this callback should not panic
	// TODO: mix audio here, maybe pull out
	//
	// Strategy to time mixing:
	// - when playback goroutine pulls pcm from pcm buf and writes to speaker buf, it empties the pcm buf (in a lock),
	//   therefore, each of the onTrack()'s below needs to have its own flag/counter, that is set when it writes to the pcm.
	//   when pcm is emptied, all track's flags are unset. therefore, each track can go:
	// 		- is any other track's flag set? if so, take the last frame from pcm buf, and mix my current frame with that,
	// 		  then overwrite that frame with the mixed one.
	// 		- if no other track's flags are set, even if mine is set, just append frame to pcm buf.
	// 	 note: could do this flag stuff with a bitfield and bitwise ops if it seems expensive (its operated on in locks)
	// 		   and the bitfield len is maxTracks (6)
	// - another strategy would be for each track to write to its own pcm buf, and have the speaker callback join frames from
	//   all present pcm bufs (one per track). so the first strategy is the onTrack doing the mixing, and this strat is the
	//   onSample doing the mixing... onSample joining is prob more robust but may be slower if pcm bufs are fragmented in the heap...
	// NOTE: should ensure that a 1:1 voice call never does any of this... only multi-user channels
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		wg.Add(1)
		defer wg.Done()

		// decoder operates on only one audio stream so init here.
		// note: could pass this in.
		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil {
			panic(fmt.Errorf("decoder init error: %w", err))
		}

		fmt.Printf("added track with id: %s, streamID: %s\n", track.ID(), track.StreamID())
		decodeBuf := make([]int16, pcmBufferSize)
		pcm := make([]int16, pcmBufferSize)
		pcmBuffers.addStream(&pcm)

		for {
			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					return // Track closed, exit loop
				}
				log.Println("PACKET READ ERR: ", readErr)
				continue // Temporary error, keep trying
			}

			// TODO: track.ID()
			// - use track.id to store pcm in a buf for only this track
			// - then mix PCM from each track before writing to speaker
			// - each track's PCM needs a lock and so does the mixing PCM
			// - just do naive mixing at first, dont do any fancy timing
			// - itd be nice to extract some of this state out into a struct with funcs

			// TODO: check for 0 samples decoded and call PLC?
			samplesDecoded, decodeErr := decoder.Decode(packet.Payload, decodeBuf)
			if decodeErr != nil {
				log.Println("DECODE ERROR: ", decodeErr.Error())
				continue
			}

			framesDecoded := samplesDecoded * NumChannels
			// Write decoded PCM to playback buffer, which malgo will pull from for playback
			pcmBuffers.mu.Lock()
			pcm = append(pcm, decodeBuf[:framesDecoded]...) // inefficient? reslice instead?
			pcmBuffers.mu.Unlock()
		}
	}
}

// createCallDevice inits, configures, and sets up the callback needed to use the speaker for peer-to-peer (1:1) voice calls.
func createCallDevice() (ctx *malgo.AllocatedContext, device *malgo.Device, pcm *CallStream, err error) {
	ctx, err = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		err = fmt.Errorf("error initializing device context: %w", err)
		return
	}

	// TODO: prealloc
	pcm = &CallStream{}

	// read into output sample buf, for output to speaker device. this fires every X milliseconds
	onSendFrames := func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		pcm.mu.Lock()
		defer pcm.mu.Unlock()

		// if there isn't yet a full sample in the pcmBuffer sent from the network
		if len(pcm.buf) < samplesToRead {
			return
		}

		// write a full sample to the speaker buffer
		copy(pOutputSample, int16ToBytes(pcm.buf[:samplesToRead]))
		pcm.buf = pcm.buf[samplesToRead:] // TODO: probably leaks
	}

	device, err = initDevice(ctx, onSendFrames)
	return
}

// createCallDevice inits, configures, and sets up the callback needed to use the speaker for multi-user voice chats (channels).
func createChannelDevice(streams *ChannelStreams) (ctx *malgo.AllocatedContext, device *malgo.Device, err error) {
	ctx, err = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		err = fmt.Errorf("error initializing device context: %w", err)
		return
	}

	// read into output sample buf, for output to speaker device. this fires every X milliseconds
	onSendFrames := func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		streams.mu.Lock()
		defer streams.mu.Unlock()

		fullBufs, ok := streams.hasFullSample(samplesToRead)
		// if there isn't yet a full sample in any of the pcm buffers sent from the network
		if !ok {
			return
		}

		mixed := streams.mixAudio(fullBufs, samplesToRead)

		// write a full mixed sample to the speaker buffer
		copy(pOutputSample, int16ToBytes(mixed[:samplesToRead]))

		// reslice all bufs that were just mixed, removing the mixed pcm from each
		for _, p := range fullBufs {
			*p = (*p)[samplesToRead:] // TODO: probably leaks
		}
	}

	device, err = initDevice(ctx, onSendFrames)
	return
}

// initDevice initalizes and starts the speaker device for playback.
func initDevice(ctx *malgo.AllocatedContext, onSendFrames malgo.DataProc) (device *malgo.Device, err error) {
	config := malgo.DefaultDeviceConfig(malgo.Playback)
	config.Playback.Format = AudioFormat
	config.Playback.Channels = NumChannels
	config.SampleRate = SampleRate
	config.PeriodSizeInMilliseconds = frameDurationMs

	// init playback device
	device, err = malgo.InitDevice(ctx.Context, config, malgo.DeviceCallbacks{
		Data: onSendFrames,
	})
	if err != nil {
		err = fmt.Errorf("error creating playback device: %w", err)
		return
	}
	if err := device.Start(); err != nil {
		err = fmt.Errorf("error starting playback device: %w", err)
	}
	return
}

// UninitPlayback uninitializes the speaker device for a 1:1 voice call. First, it attempts a graceful close
// of the PeerConnection, in order to unblock the playback goroutine, which blocks while it reads packets from the network.
// The playback wg is then waited on, while the goroutines reading from the network (RemoteTracks) complete. Regardless
// of the result of the graceful close, the malgo device is torn down.
func UninitCallPlayback(pc *webrtc.PeerConnection, ctx *malgo.AllocatedContext, device *malgo.Device, wg *sync.WaitGroup) {
	// this forces the track.ReadRTP() in audio.SetupPlayback to unblock
	if closeErr := pc.GracefulClose(); closeErr != nil {
		fmt.Printf("cannot gracefully close recipient connection: %v\n", closeErr)
	} else {
		wg.Wait()
	}

	UninitPlayback(ctx, device)
}

// uninitPlayback uninitializes the malgo playback device and frees all its resources. Ideally,
// nothing should be writing to the speaker device when this is called. This is ensured by
// closing all PeerConnections beforehand, since their RemoteTrack handlers write to the device.
func UninitPlayback(ctx *malgo.AllocatedContext, device *malgo.Device) {
	if ctx == nil {
		fmt.Println("playback ctx uninit before init")
		return
	}
	if device != nil {
		device.Uninit()
	}
	if err := ctx.Uninit(); err != nil {
		fmt.Printf("error uninitializing playback device context: %v", err)
	}
	ctx.Free()
	fmt.Println("uninit and freed playback device")
}

// int16ToBytes converts an int16 slice to a byte slice of PCM audio. TODO: can be reimpl with unsafe
func int16ToBytes(s []int16) []byte {
	result := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(v))
	}
	return result
}
