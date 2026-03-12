package audio

// state.go contains structs that encapsulate the capture and playback of opus audio over webrtc,
// in either a 1:1 voice call or 1:many voice chat format.

import (
	"io"
	"log"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

type Channel struct {
	Mic     microphone
	Speaker speaker
	streams streams
}

func NewChannel(track *webrtc.TrackLocalStaticSample) *Channel {
	return &Channel{newMicrophone(track), newSpeaker(), newStreams()}
}

// AddPeer sets an event handler on pc that decodes incoming audio.
// Decoded audio is then written to c.streams, from which the speaker goroutine
// reads and mixes with other PC's audio streams for playback.
// NOTE: DecodeFEC and DecodePLC are available for later use
// NOTE: if text remote tracks are added, this will have to not add those to audio stream struct.
func (c *Channel) AddPeer(pc *webrtc.PeerConnection) {
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
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		c.Speaker.wg.Add(1)
		defer c.Speaker.wg.Done()

		// decoder operates on only one audio stream so init here.
		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil { // panic is fine since its a startup dev error
			log.Panicf("decoder init error: %v", err)
		}

		log.Printf("added track with id: %s, streamID: %s\n", track.ID(), track.StreamID())
		decodeBuf := make([]int16, pcmBufferSize)
		pcm := make([]int16, 0, pcmBufferSize)
		c.streams.add(track.StreamID(), &pcm)

		for {
			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					c.streams.remove(track.StreamID())
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
			c.streams.mu.Lock()
			pcm = append(pcm, decodeBuf[:framesDecoded]...) // inefficient? reslice instead?
			c.streams.mu.Unlock()
		}
	})
}

// DataProc returns a callback that mixes multiple user's audio and sends it to the speaker.
func (c *Channel) DataProc() malgo.DataProc {
	// read into output sample buf, for output to speaker device.
	// this fires every [frameDurationMs]
	return func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		c.streams.mu.Lock()

		if samplesToRead > len(c.streams.mixed) {
			log.Panicf("samplesToRead >= cap(mixed")
		}

		// write a full mixed sample to the speaker buffer
		c.streams.mix(samplesToRead)
		copy(pOutputSample, int16ToBytes(c.streams.mixed[:samplesToRead]))
		c.streams.mu.Unlock()
	}
}

type Call struct {
	Mic     microphone
	Speaker speaker
	stream  stream
}

func NewCall(track *webrtc.TrackLocalStaticSample) *Call {
	return &Call{newMicrophone(track), newSpeaker(), newStream()}
}

// AddPeer sets an event handler on pc that writes incoming audio data to the speaker.
// It handles decoding opus audio for the remote track. The Call's PC
// should only have one RemoteTrack. Decoded audio is written to c.stream,
// from which the speaker goroutine reads for playback.
func (c *Call) AddPeer(pc *webrtc.PeerConnection) {
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		c.Speaker.wg.Add(1)
		defer c.Speaker.wg.Done()

		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil {
			log.Panicf("decoder init error: %v", err)
		}

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
			c.stream.mu.Lock()
			c.stream.buf = append(c.stream.buf, decodeBuf[:framesDecoded]...)
			c.stream.mu.Unlock()
		}
	})
}

// DataProc returns a callback that sends audio data to the speaker.
// https://github.com/gen2brain/malgo/blob/master/_examples/playback/playback.go
func (c *Call) DataProc() malgo.DataProc {
	return func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		c.stream.mu.Lock()
		defer c.stream.mu.Unlock()

		// if there isn't yet a full sample in the pcmBuffer sent from the network
		if len(c.stream.buf) < samplesToRead {
			return
		}

		// write a full sample to the speaker buffer
		copy(pOutputSample, int16ToBytes(c.stream.buf[:samplesToRead]))
		c.stream.buf = c.stream.buf[samplesToRead:]
	}
}
