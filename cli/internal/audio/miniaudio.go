package audio

// these cgo flags disable all miniaudio subsystems that will not be used
// https://miniaud.io/docs/manual/index.html#Building

/*
   #cgo CFLAGS: -DMA_ENABLE_ONLY_SPECIFIC_BACKENDS
   #cgo CFLAGS: -DMA_ENABLE_COREAUDIO -DMA_ENABLE_PULSEAUDIO -DMA_ENABLE_ALSA -DMA_ENABLE_JACK -DMA_ENABLE_WASAPI
   #cgo CFLAGS: -DMA_NO_DECODING -DMA_NO_ENCODING
   #cgo CFLAGS: -DMA_NO_RESOURCE_MANAGER -DMA_NO_NODE_GRAPH -DMA_NO_GENERATION
   #cgo CFLAGS: -DMA_SOUND_FLAG_NO_SPATIALIZATION -DMA_SOUND_FLAG_NO_PITCH
*/
import "C"
