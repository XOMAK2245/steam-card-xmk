VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: linux windows test clean
linux:
	mkdir -p dist/linux
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/linux/steam-card-xmk ./cmd/steam-card-xmk
	gcc -O2 -Wall -o dist/linux/steam_phantom worker/steam_phantom.c -ldl
	cp third_party/steamworks/libsteam_api.so dist/linux/

windows:
	mkdir -p dist/windows
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/windows/steam-card-xmk.exe ./cmd/steam-card-xmk
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" -o dist/windows/phantom.exe ./cmd/phantom-win
	cp third_party/steamworks/steam_api64.dll dist/windows/

test:
	go test ./...

clean:
	rm -rf dist
