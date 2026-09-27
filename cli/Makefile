# allow bash commands to be executed by recipes
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -e

# Misc notes:
# GODEBUG=inittrace=1 , shows init timings and allocs

# =========================================================================
# Detect machine info
#
# Detect OS
ifeq ($(OS),Windows_NT)
OS_NAME := windows
else
DETECTED_OS := $(shell uname -s)
ifeq ($(DETECTED_OS),Linux)
OS_NAME := linux
else ifeq ($(DETECTED_OS),Darwin)
OS_NAME := darwin
else
OS_NAME := unknown
endif
endif

# Detect Architecture
ifeq ($(OS),Windows_NT)
DETECTED_ARCH := $(PROCESSOR_ARCHITECTURE)
else
# dont need to invoke shell here... can use $$
DETECTED_ARCH := $(shell uname -m)
endif
ifeq ($(DETECTED_ARCH),x86_64)
ARCH_NAME := amd64
else ifeq ($(DETECTED_ARCH),AMD64)
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
GOEXPERIMENT := simd
GOMAXPROCS=2
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
GOAMD64 := v3  # or v4, depending on your CPU features
endif

export CC CGO_CFLAGS CGO_LDFLAGS PKG_CONFIG GOEXPERIMENT GOAMD64 OUT_DIR

.PHONY: format lint build install answer join call invite friends channel status register test escape

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
	go test -c -o $(OUT_DIR)/vogo-mixtest -tags nolibopusfile -v ./internal/audio/pcm

# run audio mixing tests and benchmarks
test-mixing: build-test-mixing
	$(OUT_DIR)/vogo-mixtest -test.bench=. -test.benchmem -test.cpu=1 -test.run=Mix

test-pade: build-test-mixing
	$(OUT_DIR)/vogo-mixtest -test.cpu=1 -test.v -test.run=TestSoftSaturate_PadeVsTanh

bench:
	go test -bench=. -benchmem -cpu=1 -tags nolibopusfile -v ./internal/audio/pcm

# shows escape analysis
escape:
	go build -gcflags="-m -m" ./... > escape.out 2>&1

# testing:
# ==============================================================================
# each shell executing vogo-client commands for testing should override
# this to '2' if they want to execute commands using the second user's config.
# ex: `export VOGOENV=2`. This cannot be a string because it is used to increment
# the port pprof is exposed on.
VOGOENV ?= 1

USER_ONE := greg
USER_TWO := tim
USER_THREE := john
CHANNEL_NAME := test

CONFIG_PATH := ./configs/dev_${VOGOENV}.toml
USER_ONE_CONFIG := ./configs/dev_1.toml
USER_TWO_CONFIG := ./configs/dev_2.toml
USER_THREE_CONFIG := ./configs/dev_3.toml

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
	$(OUT_DIR)/vogo call $(USER_TWO) --config=$(USER_ONE_CONFIG) --pprof=$(PPROF_CLIENT_AUTHORITY)

# make user_two answer user_one. fails if `make call` has not been run in another shell.
answer: build
	@echo "$(USER_TWO) answering $(USER_ONE)..."
	$(OUT_DIR)/vogo answer $(USER_ONE) --config=$(USER_TWO_CONFIG) --pprof=$(PPROF_CLIENT_AUTHORITY)

# using VOGOENV, join the channel created by `make channel`.
join: announce build
	@echo "joining $(USER_ONE)/$(CHANNEL_NAME)..."
	$(OUT_DIR)/vogo channel join $(USER_ONE)/$(CHANNEL_NAME) --config=$(CONFIG_PATH) --pprof=$(PPROF_CLIENT_AUTHORITY)

#################################################################################
# PPROF -- ex. run `make pprof-cpu` to create and view a cpu profile.
#################################################################################

.PHONY: pprof-cpu pprof-heap pprof-trace pprof-leak pprof-view profile-dir

PPROF_DIR      = ../.pprof/vogo-server
PROFILE_DIR    := $(PPROF_DIR)/$(shell date +"%m-%d_%H:%M")
PPROF_SECONDS  ?= 30
PPROF_WEB_PORT ?= 6040
LATEST_LINK    = $(PPROF_DIR)/latest

# where to look for the client's pprof endpoint.
PPROF_HOST ?= localhost
PPROF_PORT ?= 6080

# used by the pprof command
PPROF_AUTHORITY ?= $(PPROF_HOST):$(PPROF_PORT)

# if not using the default VOGOENV, increment the port the client
# exposes pprof on, in order to prevent exposing on the same port.
ifneq ($(VOGOENV),1)
PPROF_CLIENT_PORT := $$(($(PPROF_PORT) + $(VOGOENV)))
else
PPROF_CLIENT_PORT := $(PPROF_PORT)
endif
PPROF_CLIENT_AUTHORITY ?= $(PPROF_HOST):$(PPROF_CLIENT_PORT)

profile-dir:
	@mkdir -p $(PROFILE_DIR)
	@ln -sfn "$$(realpath $(PROFILE_DIR))" $(LATEST_LINK)

pprof-cpu: profile-dir
	go tool pprof -proto "http://$(PPROF_AUTHORITY)/debug/pprof/profile?seconds=$(PPROF_SECONDS)" > $(PROFILE_DIR)/cpu.prof
	go tool pprof -http=:$(PPROF_WEB_PORT) $(PROFILE_DIR)/cpu.prof

pprof-heap: profile-dir
	go tool pprof -proto "http://$(PPROF_AUTHORITY)/debug/pprof/heap?gc=1" > $(PROFILE_DIR)/heap.prof
	go tool pprof -http=:$(PPROF_WEB_PORT) $(PROFILE_DIR)/heap.prof

pprof-leak: profile-dir
	go tool pprof -proto "http://$(PPROF_AUTHORITY)/debug/pprof/goroutineleak" > $(PROFILE_DIR)/leak.prof
	go tool pprof -http=:$(PPROF_WEB_PORT) $(PROFILE_DIR)/leak.prof

# this will open up the trace on its own, no need to call pprof-view after.
pprof-trace: profile-dir
	curl -s "http://$(PPROF_AUTHORITY)/debug/pprof/trace?seconds=10" -o $(PROFILE_DIR)/trace.out
	go tool trace $(PROFILE_DIR)/trace.out

# Opens the supplied pprof file in the web browser
pprof-view:
	@echo "Viewing \$PPROF_FILE: $${PPROF_FILE:?PPROF_FILE is not set}"
	go tool pprof -http=:$(PPROF_WEB_PORT) $${PPROF_FILE}
