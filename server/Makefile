
build:
	go build -o ../bin/vogo-server main.go

run:
	go run . run

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

format:
	golangci-lint fmt

lint:
	go vet ./...
	gofmt -d -e .
	golangci-lint run -c ./../golangci.yml --fix

.PHONY: invite run build lint format
