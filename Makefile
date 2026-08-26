.PHONY: test compose-up compose-down logs format

test:
	docker build --target build .

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

logs:
	docker compose logs -f fraud-service inference-sidecar web-ui

format:
	docker run --rm -v "$(PWD):/src" -w /src golang:1.24-alpine sh -c 'gofmt -w $$(find cmd internal -name "*.go")'
