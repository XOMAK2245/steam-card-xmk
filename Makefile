VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
EMBED_LINUX := internal/embedded/bin/linux

.PHONY: linux windows worker-linux test clean

# The Linux worker is compiled first because it is embedded into the binary.
worker-linux:
	gcc -O2 -Wall -o $(EMBED_LINUX)/steam_phantom worker/steam_phantom.c -ldl

# Single self-contained binary (worker + libsteam_api.so are embedded).
linux: worker-linux
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/steam-card-xmk ./cmd/steam-card-xmk

# Single self-contained .exe (steam_api64.dll embedded; no console window).
windows:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" -o dist/steam-card-xmk.exe ./cmd/steam-card-xmk

test:
	go test ./...

clean:
	rm -rf dist $(EMBED_LINUX)/steam_phantom
