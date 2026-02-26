# TODOs

### Next:

##### CI:
- ignore or fix client opus/cgo errs
- fix all server errs
- consult upload-artifact to upload outdir with libopus.dll and README

##### Windows:
- config file path uses a fwdslash for 'vogo/vogo.toml'
- help menu shows double back slashes for each dir step
- get create-shortcut to run by double clicking it
- test out ASAN/MSAN

##### Client:
- client: put room structs in wrtc package, req structs in netw (requests.go?) or crud?
- client: add friend should 409 when friend already exists
- client: user disconnect needs to call destructor on pc and importantly the channelStream struct so that there are
  never more than 5 audio streams. and exit on PC disconnect
- client: ensure mtu=3000 is ok.
- client-side ICE sending should not fail the entire join thread and should instead retry per PC
- parallelize client sending offers when joining a room: this requires ws multiplexer on server-side
- add poll to status
- add a 'config' command that invokes default text editor (how do i do this on windows?)
- add query/status functionality to get outgoing friend requests
- remove/fix xdg config in client to match server
- impl PLC?
- ensure DTLS is working correctly and encrypting
- look here https://github.com/pion/webrtc/blob/master/examples/README.md#media-api to see info about rtcp media stats

##### Server:
- client/server: gracefully fail when channel not found (sentinel?)
- disallow user in room to join twice
- return 409 for duplicate add friend
- handle EOF: this always means the ws is closed. if it happens during normal operation this is bad... this should also not signal a successful call, instead use a sentinel
- server should listen for a client-sent ACK of successful connection, to delete the pendingConnection entry
- pull out msgHandler switch and eventHandler switch into funcs. 
- ice sending on client side needs to send a sentinel for DONE status, since room ws needs to stay open and can't 
  recv EOFs at any time (already does this with empty candidate??)

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
