package audio

import (
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
	"github.com/pion/webrtc/v4"
)

func TeardownPlaybackResources(pc *webrtc.PeerConnection, ctx *malgo.AllocatedContext, device *malgo.Device, wg *sync.WaitGroup) {
	// this forces the track.ReadRTP() in audio.SetupPlayback to unblock
	if closeErr := pc.GracefulClose(); closeErr != nil {
		fmt.Printf("cannot gracefully close recipient connection: %v\n", closeErr)
	} else {
		wg.Wait()
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

func teardownCaptureResources(ctx *malgo.AllocatedContext, device *malgo.Device) {
	if device != nil {
		device.Uninit()
	}
	if err := ctx.Uninit(); err != nil {
		fmt.Printf("error uninitializing capture device context: %v", err)
	}
	ctx.Free()
	fmt.Println("uninit and freed capture device")
}
