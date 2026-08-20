module github.com/gregriff/vogo/cli

go 1.27

require (
	github.com/gen2brain/malgo v0.11.25
	github.com/google/uuid v1.6.0
	github.com/gregriff/vogo/shared v0.0.0
	github.com/hraban/opus v0.0.0-20260708213942-bde8e4304501
	github.com/pion/interceptor v0.1.47
	github.com/pion/rtcp v1.2.17
	github.com/pion/rtp v1.10.5
	github.com/pion/webrtc/v4 v4.2.18
	github.com/spf13/cobra v1.10.1
	github.com/spf13/viper v1.21.0
	golang.org/x/net v0.50.0
	golang.org/x/sync v0.19.0
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pion/datachannel v1.6.2 // indirect
	github.com/pion/dtls/v3 v3.1.5 // indirect
	github.com/pion/ice/v4 v4.4.0 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/mdns/v2 v2.1.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/sctp v1.11.1 // indirect
	github.com/pion/sdp/v3 v3.0.19 // indirect
	github.com/pion/srtp/v3 v3.0.12 // indirect
	github.com/pion/stun/v3 v3.1.6 // indirect
	github.com/pion/transport/v4 v4.0.2 // indirect
	github.com/pion/turn/v5 v5.0.12 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.14.0 // indirect
)

replace github.com/gregriff/vogo/shared => ../shared
