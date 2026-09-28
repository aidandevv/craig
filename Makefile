.PHONY: test format tidy test-pkg build wasm extension-install extension-test extension-build dev

HOST_GOOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_GOARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
GO_IMAGE ?= golang:1.25-alpine

# Go is not assumed on the host; everything runs in a container.
test:
	docker build --target build .

build:
	mkdir -p bin
	docker run --rm -v "$(PWD):/src" -w /src \
		-e CGO_ENABLED=0 -e GOOS=$(HOST_GOOS) -e GOARCH=$(HOST_GOARCH) \
		$(GO_IMAGE) go build -trimpath -ldflags='-s -w' -o bin/craig-extension ./cmd/craig-extension

format:
	docker run --rm -v "$(PWD):/src" -w /src $(GO_IMAGE) \
		sh -c 'gofmt -w $$(find cmd internal -name "*.go")'

tidy:
	docker run --rm -v "$(PWD):/src" -w /src $(GO_IMAGE) \
		sh -c 'apk add --no-cache build-base >/dev/null && go mod tidy'

# Run one package's tests, e.g. make test-pkg PKG=./internal/rules
test-pkg:
	docker run --rm -v "$(PWD):/src" -w /src $(GO_IMAGE) \
		sh -c 'apk add --no-cache build-base >/dev/null 2>&1 && go test $(PKG) -v'

# Build the browser engine. wasm_exec.js must come from the same Go toolchain.
wasm:
	mkdir -p extension/generated
	docker run --rm -v "$(PWD):/src" -w /src -e GOOS=js -e GOARCH=wasm $(GO_IMAGE) \
		sh -c 'go build -trimpath -ldflags="-s -w" -o extension/generated/engine.wasm ./cmd/craig-wasm && cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" extension/generated/wasm_exec.js'
	@ls -lh extension/generated/engine.wasm

extension-install:
	cd extension && npm ci

extension-test:
	cd extension && npm test

extension-build:
	cd extension && npm run build

# Build the browser extension and local helper, then run the helper with safe traces.
dev: extension-build build
	./bin/craig-extension daemon --verbose
