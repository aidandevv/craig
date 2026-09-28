FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... && go test ./... && CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/craig-extension ./cmd/craig-extension
