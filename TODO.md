# TODOs

### Next:
- impl timeout on server-side for call endpoint so that a cancelled request does not cause an infinite block on a goroutine
- remove/fix xdg config in client to match server
- determine how friends should refer to eachother (include friend code?)
- impl status cmd
- impl adding friends
- impl PLC?

### Polish before release
- ensure simd is enabled
- profile cpu and mem
- 192k bitrate?
- enable caller playback!


### PRs:
- opus: OPUS_SET_SIGNAL binding on the encoder
- opus: add decoder complexity binding to enable DNN features on opus 1.5
