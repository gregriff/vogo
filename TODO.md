# TODOs

### Next:
- ensure onTrack for speaker mixes audio, pull out so that it can be added to speaker after speaker init?
- parallelize speaker init and signal its completion (its slow)
- ensure that errorChan never blocks (if multiple errors are sent to it) 
- add a edit-config command that invokes default text editor (how do i do this on windows?)
- manually impl a timeout cancel of /call ws request
- remove/fix xdg config in client to match server
- determine how friends should refer to eachother (include friend code?)
- impl status cmd
- impl adding friends
- impl PLC?
- ensure DTLS is working correctly and encrypting
- look here https://github.com/pion/webrtc/blob/master/examples/README.md#media-api to see info about rtcp media stats

### Polish before release
- ensure ws is using TLS
- ensure simd is enabled
- profile cpu and mem
- enable caller playback!


### PRs:
- opus: OPUS_SET_SIGNAL binding on the encoder
- opus: add decoder complexity binding to enable DNN features on opus 1.5
