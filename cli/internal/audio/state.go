package audio

// state.go contains structs that encapsulate the capture and playback of opus audio over webrtc,
// in either a 1:1 voice call or 1:many voice chat format.

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/gregriff/vogo/cli/internal/audio/ringbuffer"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

// audioBase contains shared state between 1:1 voice calls and multi-user channels.
type audioBase struct {
	// deviceCtx allows creation of the microphone and speaker
	deviceCtx *malgo.AllocatedContext

	// CtxChan should be sent the AllocatedContext to be shared between the mic and speaker
	CtxChan chan *malgo.AllocatedContext

	Mic     microphone
	Speaker speaker

	recvMTU int
}

func newAudioBase(track *webrtc.TrackLocalStaticSample, recvMTU int) audioBase {
	return audioBase{
		CtxChan: make(chan *malgo.AllocatedContext),
		Mic:     newMicrophone(track),
		Speaker: newSpeaker(),
		recvMTU: recvMTU,
	}
}

// CreateDeviceContext creates a malgo context that will be shared between the speaker and mic.
// It should be run in its own goroutine because this can take a while.
func (ab *audioBase) CreateDeviceContext(ctx context.Context) error {
	start := time.Now()
	defer func() {
		log.Printf("malgo context created in %v", time.Since(start))
	}()

	c, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("error initializing mic context: %w", err)
	}

	ab.deviceCtx = c

	// send context to the mic and speaker which
	// will wait on it before starting.
	ab.Mic.CtxChan <- c
	ab.Speaker.CtxChan <- c
	return nil
}

// Uninit uninitializes the cgo context that the mic and speaker rely on. It must only be called
// if both the mic and speaker are uninitialized.
func (ab *audioBase) Uninit() error {
	if ab.deviceCtx == nil {
		return nil
	}
	if err := ab.deviceCtx.Uninit(); err != nil {
		return err
	}
	ab.deviceCtx.Free()
	return nil
}

type Channel struct {
	audioBase
	streams streams
}

// NewChannel creates a new audio channel struct.
func NewChannel(track *webrtc.TrackLocalStaticSample, recvMTU int) *Channel {
	return &Channel{newAudioBase(track, recvMTU), newStreams()}
}

// AddPeer sets an event handler on pc that decodes incoming audio.
// Decoded audio is then written to c.streams, from which the speaker goroutine
// reads and mixes with other PC's audio streams for playback.
// NOTE: DecodeFEC and DecodePLC are available for later use
// NOTE: if text remote tracks are added, this will have to not add those to audio stream struct.
func (c *Channel) AddPeer(pc *webrtc.PeerConnection) {
	// note: this callback should not panic
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		// decoder operates on only one audio stream so init here.
		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil { // panic is fine since its a startup dev error
			log.Panicf("decoder init error: %v", err)
		}

		packetBuf := make([]byte, c.recvMTU)
		decodeBuf := make([]int16, pcmBufferSize)
		pcm := ringbuffer.New(ringBufferSize)
		c.streams.add(track.StreamID(), &pcm)

		for {
			r := &rtp.Packet{}

			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			_, err := ReadRTP(r, packetBuf, track, c.recvMTU)
			if err != nil {
				if err == io.EOF {
					c.streams.remove(track.StreamID())
					return // Track closed, exit loop
				}
				log.Printf("PACKET READ ERR: %v", err)
				continue // Temporary error, keep trying
			}

			// TODO: check for 0 samples decoded and call PLC?
			samplesDecoded, decodeErr := decoder.Decode(r.Payload, decodeBuf)
			if decodeErr != nil {
				log.Printf("DECODE ERROR: %v", decodeErr)
				continue
			}

			framesDecoded := samplesDecoded * NumChannels
			// Write decoded PCM to playback buffer, which malgo will pull from for playback
			c.streams.mu.Lock()
			pcm.Write(decodeBuf[:framesDecoded])
			c.streams.mu.Unlock()

			// TODO: ensure capacity doesnt continue to grow??
			// decodeBuf = decodeBuf[framesDecoded:]
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
	audioBase
	stream stream
}

// NewCall creates the state for a 1:1 voice call.
func NewCall(track *webrtc.TrackLocalStaticSample, recvMTU int) *Call {
	return &Call{newAudioBase(track, recvMTU), newStream()}
}

// AddPeer sets an event handler on pc that writes incoming audio data to the speaker.
// It handles decoding opus audio for the remote track. The Call's PC
// should only have one RemoteTrack. Decoded audio is written to c.stream,
// from which the speaker goroutine reads for playback.
func (c *Call) AddPeer(pc *webrtc.PeerConnection) {
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		decoder, err := opus.NewDecoder(SampleRate, NumChannels)
		if err != nil {
			log.Panicf("decoder init error: %v", err)
		}

		packetBuf := make([]byte, c.recvMTU)
		decodeBuf := make([]int16, pcmBufferSize)
		for {
			r := &rtp.Packet{}

			// this blocks until either a packet is fully read or the pc is shutdown (returns an io.EOF err)
			_, err := ReadRTP(r, packetBuf, track, c.recvMTU)
			if err != nil {
				if err == io.EOF {
					return // Track closed, exit loop
				}
				log.Printf("PACKET READ ERR: %v", err)
				continue // Temporary error, keep trying
			}

			// count++
			// if count%150 == 0 {
			// 	rtcp, _, err := receiver.ReadRTCP()
			// 	if err != nil {
			// 		log.Printf("rtcp recv err: %#v", err)
			// 	}
			// 	for _, r := range rtcp {
			// 		// Print a string description of the packets
			// 		if stringer, canString := r.(fmt.Stringer); canString {
			// 			fmt.Printf("Received RTCP Packet: %v", stringer.String())
			// 		}
			// 	}
			// }

			// TODO: check for 0 samples decoded and call PLC?
			samplesDecoded, decodeErr := decoder.Decode(r.Payload, decodeBuf)
			if decodeErr != nil {
				log.Println("DECODE ERROR: ", decodeErr.Error())
				continue
			}

			framesDecoded := samplesDecoded * NumChannels
			// Write decoded PCM to playback buffer, which malgo will pull from for playback
			c.stream.mu.Lock()
			c.stream.rb.Write(decodeBuf[:framesDecoded])
			c.stream.mu.Unlock()

			// TODO: ensure capacity doesnt continue to grow??
			// decodeBuf = decodeBuf[framesDecoded:]
		}
	})
}

// DataProc returns a callback that sends audio data to the speaker.
// https://github.com/gen2brain/malgo/blob/master/_examples/playback/playback.go
func (c *Call) DataProc() malgo.DataProc {
	writeBuffer := make([]int16, pcmBufferSize)
	return func(pOutputSample, _ []byte, framecount uint32) {
		samplesToRead := int(framecount) * NumChannels
		c.stream.mu.Lock()
		defer c.stream.mu.Unlock()

		// if there isn't yet a full sample in the pcmBuffer sent from the network
		if c.stream.rb.Len() < samplesToRead {
			return
		}

		// write a full sample to the speaker buffer
		_ = c.stream.rb.Read(writeBuffer)
		copy(pOutputSample, int16ToBytes(writeBuffer))
	}
}

// ReadRTP is a rewrite of webrtc.TrackRemote.ReadRTP() that reuses a
// provided buffer and rtp Packet so they don't escape to the heap.
func ReadRTP(r *rtp.Packet, buf []byte, t *webrtc.TrackRemote, recvMTU int) (interceptor.Attributes, error) {
	n, iAttrs, err := t.Read(buf)
	if err != nil {
		return nil, err
	}
	if err := r.Unmarshal(buf[:n]); err != nil {
		log.Printf("PACKET UNMARSHAL ERR: %v", err)
		return nil, err
	}
	return iAttrs, err
}
