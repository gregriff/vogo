OUT_DIR=../bin

build:
	go build -tags nolibopusfile -o $(OUT_DIR)/vogo main.go

format:
	gofmt -d -e .
	# golangci-lint fmt

lint:
	go vet ./...
	golangci-lint run -c ./../golangci.yml --fix

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
	$(OUT_DIR)/vogo channel join $(USER_ONE)/$(CHANNEL_NAME) --config=$(CONFIG_PATH)

.PHONY: format lint build answer join call invite friends channel status register
