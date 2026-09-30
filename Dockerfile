FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git python3 xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 --shell /usr/sbin/nologin app

RUN curl -fsSL https://raw.githubusercontent.com/arduino/arduino-cli/master/install.sh \
    | VERSION=1.5.1 BINDIR=/usr/local/bin sh

USER app
RUN arduino-cli core update-index \
    && arduino-cli core install arduino:avr@1.8.8 \
    && arduino-cli core install esp32:esp32@3.3.12

WORKDIR /app
COPY --from=builder /out/server /app/server
RUN mkdir -p /app/data/projects

ENV HOST=0.0.0.0 \
    PORT=8080 \
    PROJECT_DIR=/app/data/projects \
    ARDUINO_CLI_PATH=/usr/local/bin/arduino-cli

EXPOSE 8080
ENTRYPOINT ["/app/server"]
