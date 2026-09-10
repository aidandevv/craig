.PHONY: test format tidy test-pkg build extension-install extension-test extension-build

HOST_GOOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_GOARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')

# Go is not assumed on the host; everything runs in a container.
test:
	docker build --target build .

build:
	mkdir -p bin
	docker run --rm -v "$(PWD):/src" -w /src \
		-e CGO_ENABLED=0 -e GOOS=$(HOST_GOOS) -e GOARCH=$(HOST_GOARCH) \
		golang:1.24-alpine go build -trimpath -ldflags='-s -w' -o bin/craig-extension ./cmd/craig-extension

format:
	docker run --rm -v "$(PWD):/src" -w /src golang:1.24-alpine \
		sh -c 'gofmt -w $$(find cmd internal -name "*.go")'

tidy:
	docker run --rm -v "$(PWD):/src" -w /src golang:1.24-alpine \
		sh -c 'apk add --no-cache build-base >/dev/null && go mod tidy'

# Run one package's tests, e.g. make test-pkg PKG=./internal/rules
test-pkg:
	docker run --rm -v "$(PWD):/src" -w /src golang:1.24-alpine \
		sh -c 'apk add --no-cache build-base >/dev/null 2>&1 && go test $(PKG) -v'

extension-install:
	cd extension && npm ci

extension-test:
	cd extension && npm test

extension-build:
	cd extension && npm run build
