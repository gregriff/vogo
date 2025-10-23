
build:
	go build -o ../bin/vogo main.go

# testing
CALLER_NAME=greg
ANSWERER_NAME=tim
CALLER_CONFIG_PATH=./configs/dev.caller.toml
ANSWERER_CONFIG_PATH=./configs/dev.answerer.toml

register-caller:
	go run . register --code=$(CODE) --config=$(CALLER_CONFIG_PATH)

register-answerer:
	go run . register --code=$(CODE) --config=$(ANSWERER_CONFIG_PATH)

call:
	go run . call $(ANSWERER_NAME) --config=$(CALLER_CONFIG_PATH)

answer:
	go run . answer $(CALLER_NAME) --config=$(ANSWERER_CONFIG_PATH)
