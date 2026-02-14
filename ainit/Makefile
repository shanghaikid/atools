BINARY    := ainit
PREFIX    := /usr/local/bin
GOFLAGS   := -trimpath
LDFLAGS   := -s -w

.PHONY: build install uninstall test vet clean

build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) .

install: build
	install -d $(PREFIX)
	install -m 755 $(BINARY) $(PREFIX)/$(BINARY)
	@echo "Installed $(BINARY) to $(PREFIX)/$(BINARY)"

uninstall:
	rm -f $(PREFIX)/$(BINARY)
	@echo "Removed $(PREFIX)/$(BINARY)"

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
