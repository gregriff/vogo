
build:
	go build -o ../bin/vogo-server main.go

run:
	go run . run

invite:
	go run . create-invite

# db scripts
PGDATABASE=vogo-server
drop-db:
	dropdb $(PGDATABASE)

create-db:
	createdb $(PGDATABASE)

.PHONY: invite run build
