OUT_DIR=../bin

build:
	go build -tags nolibopusfile -o $(OUT_DIR)/vogo main.go

# testing:
# ==============================================================================
# each shell executing vogo-client commands for testing should override
# this to 'answerer' if they want to execute commands using the answerer config.
# ex: `export VOGOENV=answerer`
VOGOENV ?= caller

CHANNEL_NAME := test
CALLER_NAME := greg
ANSWERER_NAME := tim

CONFIG_PATH := ./configs/dev.${VOGOENV}.toml
CALLER_CONFIG_PATH := ./configs/dev.caller.toml
ANSWERER_CONFIG_PATH := ./configs/dev.answerer.toml

# note: X=1 uses deferred evaluation, so X could change every time its used

announce:
	@echo "using VOGOENV=$(VOGOENV)"

# registers a new vogo user, assuming a running vogo server defined in the config file
register: announce build
	$(OUT_DIR)/vogo register --code=$(CODE) --config=$(CONFIG_PATH)

# fetch status from vogo server
status: announce build
	$(OUT_DIR)/vogo status --config=$(CONFIG_PATH)

# make the two test users friends. run once.
friends: build
	@echo "running command to make $(CALLER_NAME) and $(ANSWERER_NAME) friends..."
	$(OUT_DIR)/vogo add $(CALLER_NAME) --config=$(ANSWERER_CONFIG_PATH)
	$(OUT_DIR)/vogo add $(ANSWERER_NAME) --config=$(CALLER_CONFIG_PATH)

# create a testing channel using the caller test client.
channel: build
	$(OUT_DIR)/vogo channel create $(CHANNEL_NAME) --config=$(CALLER_CONFIG_PATH)

# invite the answerer to the caller client's channel. assumes `make channel` has been run.
invite: build
	@echo "inviting $(ANSWERER_NAME) to $(CALLER_NAME)'s $(CHANNEL_NAME) channel..."
	$(OUT_DIR)/vogo channel invite $(ANSWERER_NAME) $(CHANNEL_NAME) --config=$(CALLER_CONFIG_PATH)

# make caller call answerer.
call: build
	@echo "$(CALLER_NAME) calling $(ANSWERER_NAME)..."
	$(OUT_DIR)/vogo call $(ANSWERER_NAME) --config=$(CALLER_CONFIG_PATH)

# make answerer answer caller. fails if `make call` has not been run in another shell.
answer: build
	@echo "$(ANSWERER_NAME) answering $(CALLER_NAME)..."
	$(OUT_DIR)/vogo answer $(CALLER_NAME) --config=$(ANSWERER_CONFIG_PATH)

# using VOGOENV, join the channel created by `make channel`.
join: announce build
	@echo "joining $(CALLER_NAME)/$(CHANNEL_NAME)..."
	$(OUT_DIR)/vogo channel join $(CALLER_NAME)/$(CHANNEL_NAME) --config=$(CONFIG_PATH)

# lint:
# 	go vet ./...
# 	gofmt -d -e .
# 	golangci-lint run
