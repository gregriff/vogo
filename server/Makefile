
build:
	go build -o ../bin/vogo-server main.go

run:
	go run . run

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

.PHONY: build run serve invite format lint create-db drop-db
