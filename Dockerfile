# Multi-stage build for DNS container
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy source
COPY . .

# Build DNS server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o dnsserver \
    ./cmd/dnsserver

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates netcat-openbsd

WORKDIR /app

# Copy binary and configs
COPY --from=builder /build/dnsserver /app/dnsserver
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Non-root user
RUN addgroup -g 1000 dnsuser && \
    adduser -D -u 1000 -G dnsuser dnsuser && \
    chown -R dnsuser:dnsuser /app

USER dnsuser

EXPOSE 9000/udp 9001/tcp

ENTRYPOINT ["/app/dnsserver"]
