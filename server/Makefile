
build:
	go build -o ../bin/vogo-server main.go

run:
	go run . run --pprof=localhost:6061

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

PPROF_DIR = ../.pprof/vogo-server
PROFILE_DIR ?= ${PPROF_DIR}/$(shell date +"%m-%d_%H:%M")
PPROF_SECONDS ?= 60
PPROF_PORT ?= 6090

pprof-cpu:
	mkdir -p $(PROFILE_DIR)
	go tool pprof -proto http://localhost:6061/debug/pprof/profile\?seconds\=$(PPROF_SECONDS) > $(PROFILE_DIR)/cpu.prof

pprof-goroutine:
	mkdir -p $(PROFILE_DIR)
	go tool pprof -proto http://localhost:6061/debug/pprof/goroutine > $(PROFILE_DIR)/goroutine.prof

pprof-heap:
	mkdir -p $(PROFILE_DIR)
	go tool pprof -proto http://localhost:6061/debug/pprof/heap > $(PROFILE_DIR)/heap.prof

# Opens an existing pprof file in the web browser
pprof-view:
	@echo "Viewing \$PPROF_FILE: $${PPROF_FILE:?PPROF_FILE is not set}"
	go tool pprof -http=:$(PPROF_PORT) $${PPROF_FILE}

.PHONY: build run serve invite format lint create-db drop-db
