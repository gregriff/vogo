GOEXPERIMENT=greenteagc
OUT_DIR=../bin

build:
	go build -tags nolibopusfile -o $(OUT_DIR)/vogo main.go

# testing
CALLER_NAME := greg
ANSWERER_NAME := tim
CALLER_CONFIG_PATH := ./configs/dev.caller.toml
ANSWERER_CONFIG_PATH := ./configs/dev.answerer.toml

# note: X=1 uses deferred evaluation, so X could change every time its used

register-caller:
	go run . register --code=$(CODE) --config=$(CALLER_CONFIG_PATH)

register-answerer:
	go run . register --code=$(CODE) --config=$(ANSWERER_CONFIG_PATH)

call: build
	$(OUT_DIR)/vogo call $(ANSWERER_NAME) --config=$(CALLER_CONFIG_PATH)

answer: build
	$(OUT_DIR)/vogo answer $(CALLER_NAME) --config=$(ANSWERER_CONFIG_PATH)

# lint:
# 	go vet ./...
# 	gofmt -d -e .
# 	golangci-lint run
