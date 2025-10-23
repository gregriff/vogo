# for macos only
XDG_DATA_HOME="$(HOME)/Library/Application\ Support"

build:
	go build -o ../bin/vogo-server main.go

run:
	go run . run

invite:
	go run . create-invite

wipe-database:
	rm "$(XDG_DATA_HOME)/vogo-server/vogo-server.sqlite"
