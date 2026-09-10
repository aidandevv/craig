.PHONY: test format tidy test-pkg

# Go is not assumed on the host; everything runs in a container.
test:
	docker build --target build .

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
