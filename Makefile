BINARY  := plexmatch-generator
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet clean rpi rpi32 rpi-zero release

## build: compile for the current machine
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

## test: run the unit tests with the race detector
test:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## rpi: build a static binary for 64-bit Raspberry Pi OS (Pi 3/4/5) -> bin/plexmatch-generator-linux-arm64
rpi:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux-arm64 .

## rpi32: build for 32-bit Raspberry Pi OS (Pi 2/3, ARMv7)
rpi32:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux-armv7 .

## rpi-zero: build for Pi Zero / Pi 1 (ARMv6)
rpi-zero:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux-armv6 .

## release: build every Raspberry Pi target
release: rpi rpi32 rpi-zero

## clean: remove build artefacts
clean:
	rm -rf bin

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
