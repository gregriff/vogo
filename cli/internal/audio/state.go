package audio

// state.go contains the public interfaces to the audio package. It's exported structs
// encapsulate the capture and playback of opus audio over webrtc, in either a 1:1 voice call
// or 1:many voice chat format.

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

type ChannelState struct {
	CaptureTrack *webrtc.TrackLocalStaticSample
	capture      *capture
	playback     *playback
	pcmBufs      *channelStreams
}

func NewChannelState(track *webrtc.TrackLocalStaticSample) *ChannelState {
	return &ChannelState{
		CaptureTrack: track,
		capture:      newCapture(),
		playback:     newPlayback(),
		pcmBufs:      newChannelStreams(),
	}
}

// InitPlayback inits, configures, and sets up the speaker playback device with malgo,
// with the callback needed to use the speaker for multi-user voice chats (channels).
func (cs *ChannelState) InitPlayback() error {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	cs.playback.ctx = ctx
	if err != nil {
		return fmt.Errorf("error initializing device context: %w", err)
	}
	user := os.Getenv("VOGOENV")
	streams := cs.pcmBufs

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
		if user != "two" { // temp for testing
			copy(pOutputSample, int16ToBytes(mixed[:samplesToRead]))
		}

		// reslice all bufs that were just mixed, removing the mixed pcm from each
		for _, p := range fullBufs {
			*p = (*p)[samplesToRead:] // TODO: probably leaks
		}
	}

	if err := cs.playback.init(onSendFrames); err != nil {
		return err
	}
	// cs.Playback.Initialized <- struct{}{}
	close(cs.playback.initialized)
	return nil
}

// UninitPlayback waits for all PeerConnections to close, so that nothing
// is writing to the speaker devices when they are uninitialized.
func (cs *ChannelState) UninitPlayback() {
	cs.playback.wg.Wait()
	cs.playback.uninit()
}

// OnRemoteTrack handles decoding decoding opus audio for each remote track of a PeerConnection. It
// should be attached to each PeerConnection using pc.OnTrack(). For vogo, each PC should only have one
// RemoteTrack. Decoded audio is written to pcmBuffers, from which the speaker goroutine reads and mixes
// with other PC's audio streams for playback. This should only be used for multi-user voice chat channels/rooms.
// NOTE: DecodeFEC and DecodePLC are available for later use
// NOTE: if text remote tracks are added, this will have to not add those to audio stream struct.
func (cs *ChannelState) OnRemoteTrack() func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// this is where the decoder writes pcm from the network
	// note: this callback should not panic
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
	//   all present pcm bufs (one per track). so the first strategy is the onTrack doing the mixing, and this start is the
	//   onSample doing the mixing... onSample joining is prob more robust but may be slower if pcm bufs are fragmented in the heap...
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		cs.playback.wg.Add(1)
		defer cs.playback.wg.Done()

		// decoder operates on only one audio stream so init here.
		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil {
			panic(fmt.Errorf("decoder init error: %w", err))
		}

		log.Printf("added track with id: %s, streamID: %s\n", track.ID(), track.StreamID())
		decodeBuf := make([]int16, pcmBufferSize)
		pcm := make([]int16, pcmBufferSize)
		streams := cs.pcmBufs
		streams.addStream(&pcm)

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
			streams.mu.Lock()
			pcm = append(pcm, decodeBuf[:framesDecoded]...) // inefficient? reslice instead?
			streams.mu.Unlock()
		}
	}
}

// PlaybackInitialized returns the channel to notify the caller when the playback system
// is fully initialized. Since playback initialization is slow, this allows the caller to do
// it asynchronously and wait for its completion.
func (cs *ChannelState) PlaybackInitialized() <-chan struct{} {
	return cs.playback.initialized
}

type CallState struct {
	CaptureTrack *webrtc.TrackLocalStaticSample
	playback     *playback
	pcm          *callStream
}

func NewCallState(track *webrtc.TrackLocalStaticSample) *CallState {
	return &CallState{
		CaptureTrack: track,
		playback:     newPlayback(),
		pcm:          &callStream{},
	}
}

// SetupPlayback initializes the playback device with malgo, and defines the callback that is run per remote-track, that
// reads the audio from the network and places it in the buffer for the playback device to read from. This function
// is used by voice calls (1:1)
// TODO: should this return the pcm stuff so that onTrack can be called by other goroutines once PCs are created?
func (cs *CallState) InitPlayback(pc *webrtc.PeerConnection) error {
	if err := cs.initSpeaker(); err != nil {
		return fmt.Errorf("error initializing playback device: %w", err)
	}

	decoder, err := opus.NewDecoder(SampleRate, NumChannels)
	if err != nil {
		return fmt.Errorf("decoder init error: %w", err)
	}

	// this func runs for every remote track connected to this peer connection
	// this is where the decoder writes pcm from the network
	// note: this callback should not panic
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		cs.playback.wg.Add(1)
		defer cs.playback.wg.Done()

		pcm := cs.pcm
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
	close(cs.playback.initialized)
	return nil
}

// initSpeaker inits, configures, and sets up the callback needed to use the speaker for peer-to-peer (1:1) voice calls.
func (cs *CallState) initSpeaker() error {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	cs.playback.ctx = ctx
	if err != nil {
		return fmt.Errorf("error initializing device context: %w", err)
	}

	pcm := cs.pcm

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

	if err := cs.playback.init(onSendFrames); err != nil {
		return err
	}
	return nil
}

func (cs *CallState) UninitPlayback(pc *webrtc.PeerConnection) {
	// this forces the track.ReadRTP() in audio.SetupPlayback to unblock
	if closeErr := pc.GracefulClose(); closeErr != nil {
		log.Printf("cannot gracefully close recipient connection: %v\n", closeErr)
	} else {
		cs.playback.wg.Wait()
	}

	cs.playback.uninit()
}
