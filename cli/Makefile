# allow bash commands to be executed by recipes
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -e

# =========================================================================
# Detect machine info
#
# Detect OS
ifeq ($(OS),Windows_NT)
	DETECTED_OS := Windows
else
	DETECTED_OS := $(shell uname -s)
endif
ifeq ($(DETECTED_OS),Linux)
	OS_NAME := linux
else ifeq ($(DETECTED_OS),Darwin)
	OS_NAME := darwin
else ifeq ($(DETECTED_OS),Windows)
	OS_NAME := windows
else
	OS_NAME := unknown
endif

# Detect Architecture
ifeq ($(OS),Windows_NT)
	DETECTED_ARCH := $(PROCESSOR_ARCHITECTURE)
else
	DETECTED_ARCH := $(shell uname -m)
endif
ifeq ($(DETECTED_ARCH),x86_64)
	ARCH_NAME := amd64
else ifeq ($(DETECTED_ARCH),aarch64)
	ARCH_NAME := arm64
else ifeq ($(DETECTED_ARCH),arm64)
	ARCH_NAME := arm64
else
	ARCH_NAME := unknown
endif

# ---
ifeq ($(OS_NAME),unknown)
	$(error Unsupported OS detected: $(DETECTED_OS))
endif
ifeq ($(ARCH_NAME),unknown)
	$(error Unsupported architecture detected: $(DETECTED_ARCH))
endif
# ---

.PHONY: info
info:
	@echo "OS: $(OS_NAME)"
	@echo "ARCH: $(ARCH_NAME)"

# =========================================================================

# global env vars
GOARCH := $(ARCH_NAME)
GOOS := $(OS_NAME)
GOEXPERIMENT := simd
OUT_DIR := ./bin

# os env vars
# -------------------------------
# on windows with mingw, I didn't use a package manager to install libopus,
# instead manually compiling it and placing the artifacts in these paths.
# Refer to BUILDING.md for instructions.
ifeq ($(OS_NAME),windows)
	CC := gcc
	CGO_CFLAGS += -I/c/deps/opus/include/opus
	CGO_LDFLAGS += -L/c/deps/opus/lib -lopus
	PKG_CONFIG := true
endif
# it is assumed on darwin and linux that libopus will have been
# installed already with a package manager and findable.

# cpu arch env vars
ifeq ($(ARCH_NAME),amd64)
	GOAMD64 := v4  # or v3, depending on your CPU features
endif

export CC CGO_CFLAGS CGO_LDFLAGS PKG_CONFIG GOEXPERIMENT GOARCH GOAMD64 GOOS OUT_DIR

# view asm: `go tool objdump -s "func_name" bin/vogo`

build:
	go build -tags nolibopusfile -o $(OUT_DIR)/vogo main.go

install:
	go install -tags nolibopusfile

format:
	gofmt -d -e .
	# golangci-lint fmt

lint:
	go vet ./...
	golangci-lint run -c ./../golangci.yml --fix

test:
	go test -tags nolibopusfile -v ./...

build-test-mixing:
	go test -c -o $(OUT_DIR)/vogo-mixtest -tags nolibopusfile -v ./internal/audio

# run audio mixing tests and benchmarks
test-mixing: build-test-mixing
	$(OUT_DIR)/vogo-mixtest -test.bench=. -test.benchmem -test.cpu=1 -test.run=Mix

test-pade: build-test-mixing
	$(OUT_DIR)/vogo-mixtest -test.cpu=1 -test.v -test.run=TestSoftSaturate_PadeVsTanh

bench:
	go test -bench=. -benchmem -cpu=1 -tags nolibopusfile -v ./internal/audio

# shows escape analysis
escape:
	go build -gcflags="-m -m" ./... > escape.out 2>&1

# testing:
# ==============================================================================
# each shell executing vogo-client commands for testing should override
# this to 'two' if they want to execute commands using the second user's config.
# ex: `export VOGOENV=two`
VOGOENV ?= one

USER_ONE := greg
USER_TWO := tim
USER_THREE := john
CHANNEL_NAME := test

CONFIG_PATH := ./configs/dev.${VOGOENV}.toml
USER_ONE_CONFIG := ./configs/dev.one.toml
USER_TWO_CONFIG := ./configs/dev.two.toml
USER_THREE_CONFIG := ./configs/dev.three.toml

# note: X=1 uses deferred evaluation, so X could change every time its used

announce:
	@echo "using VOGOENV=$(VOGOENV)"

# registers a new vogo user, assuming a running vogo server defined in the config file
register: announce build
	$(OUT_DIR)/vogo register --code=$(CODE) --config=$(CONFIG_PATH)

# fetch status from vogo server
status: announce build
	$(OUT_DIR)/vogo status --config=$(CONFIG_PATH)

# make the test users friends. run once.
friends: build
	@echo "running command to make test users friends..."
	$(OUT_DIR)/vogo add $(USER_ONE) --config=$(USER_TWO_CONFIG)
	$(OUT_DIR)/vogo add $(USER_TWO) --config=$(USER_ONE_CONFIG)

	$(OUT_DIR)/vogo add $(USER_ONE) --config=$(USER_THREE_CONFIG)
	$(OUT_DIR)/vogo add $(USER_THREE) --config=$(USER_ONE_CONFIG)

	$(OUT_DIR)/vogo add $(USER_TWO) --config=$(USER_THREE_CONFIG)
	$(OUT_DIR)/vogo add $(USER_THREE) --config=$(USER_TWO_CONFIG)

# create a testing channel using the user_one test client.
channel: build
	$(OUT_DIR)/vogo channel create $(CHANNEL_NAME) --config=$(USER_ONE_CONFIG)

# invite users to user_one's channel. assumes `make channel` has been run.
invite: build
	@echo "inviting $(USER_TWO) and $(USER_THREE) to $(USER_ONE)'s $(CHANNEL_NAME) channel..."
	$(OUT_DIR)/vogo channel invite $(USER_TWO) $(CHANNEL_NAME) --config=$(USER_ONE_CONFIG)
	$(OUT_DIR)/vogo channel invite $(USER_THREE) $(CHANNEL_NAME) --config=$(USER_ONE_CONFIG)

# make user_one call user_two.
call: build
	@echo "$(USER_ONE) calling $(USER_TWO)..."
	$(OUT_DIR)/vogo call $(USER_TWO) --config=$(USER_ONE_CONFIG)

# make user_two answer user_one. fails if `make call` has not been run in another shell.
answer: build
	@echo "$(USER_TWO) answering $(USER_ONE)..."
	$(OUT_DIR)/vogo answer $(USER_ONE) --config=$(USER_TWO_CONFIG)

# using VOGOENV, join the channel created by `make channel`.
join: announce build
	@echo "joining $(USER_ONE)/$(CHANNEL_NAME)..."
	$(OUT_DIR)/vogo channel join $(USER_ONE)/$(CHANNEL_NAME) --config=$(CONFIG_PATH) --pprof=localhost:6080

PPROF_DIR = ../.pprof/vogo
PROFILE_DIR ?= ${PPROF_DIR}/$(shell date +"%m-%d_%H:%M")
PPROF_SECONDS ?= 30
PPROF_PORT ?= 6040

pprof-cpu:
	mkdir -p $(PROFILE_DIR)
	go tool pprof -proto http://localhost:6080/debug/pprof/profile\?seconds\=$(PPROF_SECONDS) > $(PROFILE_DIR)/cpu.prof

pprof-heap:
	mkdir -p $(PROFILE_DIR)
	go tool pprof -proto http://localhost:6080/debug/pprof/heap?gc=1 > $(PROFILE_DIR)/heap.prof

pprof-trace:
	mkdir -p $(PROFILE_DIR)
	curl -s "http://localhost:6080/debug/pprof/trace?seconds=10" -o $(PROFILE_DIR)/trace.out
	go tool trace $(PROFILE_DIR)/trace.out
	# go tool pprof -proto http://localhost:6080/debug/pprof/trace?seconds=10 > $(PROFILE_DIR)/trace.out

# Opens an existing pprof file in the web browser
pprof-view:
	@echo "Viewing \$PPROF_FILE: $${PPROF_FILE:?PPROF_FILE is not set}"
	go tool pprof -http=:$(PPROF_PORT) $${PPROF_FILE}

trace-view:



.PHONY: format lint build install answer join call invite friends channel status register test escape
