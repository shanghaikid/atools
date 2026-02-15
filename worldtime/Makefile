BINARY := worldtime
SRC    := ./cmd/worldtime

.PHONY: build install test vet clean

build:
	go build -o $(BINARY) $(SRC)

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
