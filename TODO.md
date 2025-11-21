# TODOs

### Next:
- ensure server cancels req when client closes it
- ensure onTrack for speaker mixes audio, pull out so that it can be added to speaker after speaker init?
- ensure that errorChan never blocks (if multiple errors are sent to it) 
- add a 'config' command that invokes default text editor (how do i do this on windows?)
- 
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
- add a updater service that runs async upon client init that checks the vogo github releases for a newer release, and prompts to run
  a new updater binary, that downloads new release and replaces current bin. ensure this preserves symlinks/shortcuts from og bin
- see if shell completion can be reran after every 'vogo status', to autocomplete the 'vogo answer' command to use the caller's name
- enable caller playback!


### PRs:
- opus: OPUS_SET_SIGNAL binding on the encoder
- opus: add decoder complexity binding to enable DNN features on opus 1.5
