FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./... && CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/apartment-hunter ./cmd/apartment-hunter

FROM alpine:3.21
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/apartment-hunter /app/apartment-hunter
RUN mkdir /app/data && chown -R app:app /app
USER app
EXPOSE 8080 9090
ENTRYPOINT ["/app/apartment-hunter"]
