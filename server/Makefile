# allow bash commands to be executed by recipes
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -e

.PHONY: build run serve invite format lint create-db drop-db

GOMAXPROCS=2

build:
	go build -o ../bin/vogo-server main.go

run:
	go run . run --pprof=$(PPROF_AUTHORITY)

format:
	# golangci-lint fmt
	gofmt -d -e .

lint:
	go vet ./...
	golangci-lint run -c ./../golangci.yml --fix

serve:
	go run . run --config=./configs/vogo-server.toml

invite:
	go run . create-invite

# db scripts
PGDATABASE=vogo-server
drop-db:
	dropdb $(PGDATABASE)

create-db:
	createdb $(PGDATABASE)

#################################################################################
# PPROF -- ex. run `make pprof-cpu` to create and view a cpu profile.
#################################################################################

.PHONY: pprof-cpu pprof-heap pprof-trace pprof-leak pprof-view profile-dir

# where server exposes pprof endpoint.
PPROF_AUTHORITY ?= localhost:6061

PPROF_DIR      = ../.pprof/vogo-server
PROFILE_DIR   := $(PPROF_DIR)/$(shell date +"%m-%d_%H:%M")
PPROF_SECONDS ?= 30
PPROF_PORT    ?= 6090
LATEST_LINK    = $(PPROF_DIR)/latest

profile-dir:
	@mkdir -p $(PROFILE_DIR)
	@ln -sfn "$$(realpath $(PROFILE_DIR))" $(LATEST_LINK)

pprof-cpu: profile-dir
	go tool pprof -proto "http://$(PPROF_AUTHORITY)/debug/pprof/profile?seconds=$(PPROF_SECONDS)" > $(PROFILE_DIR)/cpu.prof
	go tool pprof -http=:$(PPROF_PORT) $(PROFILE_DIR)/cpu.prof

pprof-heap: profile-dir
	go tool pprof -proto "http://$(PPROF_AUTHORITY)/debug/pprof/heap?gc=1" > $(PROFILE_DIR)/heap.prof
	go tool pprof -http=:$(PPROF_PORT) $(PROFILE_DIR)/heap.prof

pprof-leak: profile-dir
	go tool pprof -proto "http://$(PPROF_AUTHORITY)/debug/pprof/goroutineleak" > $(PROFILE_DIR)/leak.prof
	go tool pprof -http=:$(PPROF_PORT) $(PROFILE_DIR)/leak.prof

# this will open up the trace on its own, no need to call pprof-view after.
pprof-trace: profile-dir
	curl -s "http://$(PPROF_AUTHORITY)/debug/pprof/trace?seconds=10" -o $(PROFILE_DIR)/trace.out
	go tool trace $(PROFILE_DIR)/trace.out

# Opens the supplied pprof file in the web browser
pprof-view:
	@echo "Viewing \$PPROF_FILE: $${PPROF_FILE:?PPROF_FILE is not set}"
	go tool pprof -http=:$(PPROF_PORT) $${PPROF_FILE}
