package audio

// state.go contains structs that encapsulate the capture and playback of opus audio over webrtc,
// in either a 1:1 voice call or 1:many voice chat format.

import (
	"io"
	"log"
	"os"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

type Channel struct {
	Mic     microphone
	Speaker speaker
	streams *streams
}

func NewChannel(track *webrtc.TrackLocalStaticSample) *Channel {
	return &Channel{
		Mic:     newMicrophone(track),
		Speaker: newSpeaker(),
		streams: newStreams(),
	}
}

// AddPeer sets an event handler on pc that writes incoming audio data to the speaker.
func (c *Channel) AddPeer(pc *webrtc.PeerConnection) {
	pc.OnTrack(c.onRemoteTrack())
}

// onRemoteTrack returns the function to be run for each (audio) webrtc.TrackRemote for each
// of this Channel's registered PeerConnections. It handles decoding opus audio for each remote track.
// For vogo, each PC should only have one RemoteTrack. Decoded audio is written to c.streams,
// from which the speaker goroutine reads and mixes with other PC's audio streams for playback.
// NOTE: DecodeFEC and DecodePLC are available for later use
// NOTE: if text remote tracks are added, this will have to not add those to audio stream struct.
func (c *Channel) onRemoteTrack() func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// Strategy to time mixing:
	// - when playback goroutine pulls pcm from pcm buf and writes to speaker buf, it empties the pcm buf (in a lock),
	//   therefore, each of the onTrack()'s below needs to have its own flag/counter, that is set when it writes to the pcm.
	//   when pcm is emptied, all track's flags are unset. therefore, each track can go:
	// 		- is any other track's flag set? if so, take the last frame from pcm buf, and mix my current frame with that,
	// 		  then overwrite that frame with the mixed one.
	// 		- if no other track's flags are set, even if mine is set, just append frame to pcm buf.
	// 	 note: could do this flag stuff with a bitfield and bitwise ops if it seems expensive (its operated on in locks)
	// 		   and the bitfield len is maxTracks (6)

	// note: this callback should not panic
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		c.Speaker.wg.Add(1)
		defer c.Speaker.wg.Done()

		// decoder operates on only one audio stream so init here.
		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil { // panic is fine since its a startup dev error
			log.Panicf("decoder init error: %v", err)
		}

		log.Printf("added track with id: %s, streamID: %s\n", track.ID(), track.StreamID())
		decodeBuf := make([]int16, pcmBufferSize)
		pcm := make([]int16, pcmBufferSize)
		streams := c.streams
		streams.add(&pcm)

		for {
			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					return // Track closed, exit loop
				}
				log.Printf("PACKET READ ERR: %v", readErr)
				continue // Temporary error, keep trying
			}

			// TODO: check for 0 samples decoded and call PLC?
			samplesDecoded, decodeErr := decoder.Decode(packet.Payload, decodeBuf)
			if decodeErr != nil {
				log.Printf("DECODE ERROR: %v", decodeErr)
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

// DataProc mixes multiple user's audio and sends it to the speaker.
func (c *Channel) DataProc() malgo.DataProc {
	user := os.Getenv("VOGOENV")
	streams := c.streams

	// read into output sample buf, for output to speaker device.
	// this fires every [frameDurationMs]
	return func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		streams.mu.Lock()

		fullBufs, ok := streams.hasFullSample(samplesToRead)
		// if there isn't yet a full sample in any of the pcm buffers sent from the network
		if !ok {
			streams.mu.Unlock()
			return
		}

		mixed := streams.mix(fullBufs, samplesToRead)

		// write a full mixed sample to the speaker buffer
		if user != "two" { // temp for testing
			copy(pOutputSample, int16ToBytes(mixed[:samplesToRead]))
		}

		// reslice all bufs that were just mixed, removing the mixed pcm from each
		for _, p := range fullBufs {
			*p = (*p)[samplesToRead:] // TODO: probably leaks
		}
		streams.mu.Unlock()
	}
}

type Call struct {
	Mic     microphone
	Speaker speaker
	stream  *stream
}

func NewCall(track *webrtc.TrackLocalStaticSample) *Call {
	return &Call{
		Mic:     newMicrophone(track),
		Speaker: newSpeaker(),
		stream:  &stream{},
	}
}

// AddPeer sets an event handler on pc that writes incoming audio data to the speaker.
func (c *Call) AddPeer(pc *webrtc.PeerConnection) {
	pc.OnTrack(c.onRemoteTrack())
}

// onRemoteTrack returns the function to be run for each (audio) webrtc.TrackRemote for the
// Call's PeerConnection. It handles decoding opus audio for the remote track. The Call's PC
// should only have one RemoteTrack. Decoded audio is written to c.stream,
// from which the speaker goroutine reads for playback.
func (c *Call) onRemoteTrack() func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// this func runs for every remote track connected to this peer connection
	// this is where the decoder writes pcm from the network
	// note: this callback should not panic
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		c.Speaker.wg.Add(1)
		defer c.Speaker.wg.Done()

		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil {
			log.Panicf("decoder init error: %w", err)
		}

		pcm := c.stream
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
	}
}

// DataProc sends audio data to the speaker.
// https://github.com/gen2brain/malgo/blob/master/_examples/playback/playback.go
func (c *Call) DataProc() malgo.DataProc {
	pcm := c.stream

	return func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		pcm.mu.Lock()

		// if there isn't yet a full sample in the pcmBuffer sent from the network
		if len(pcm.buf) < samplesToRead {
			pcm.mu.Unlock()
			return
		}

		// write a full sample to the speaker buffer
		copy(pOutputSample, int16ToBytes(pcm.buf[:samplesToRead]))
		pcm.buf = pcm.buf[samplesToRead:] // TODO: probably leaks
		pcm.mu.Unlock()
	}
}
