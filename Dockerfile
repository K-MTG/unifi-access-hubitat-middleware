# Stage 1: Build the Go binary
FROM golang:1.25.4-bookworm AS builder

WORKDIR /app

# Copy go.mod and go.sum first for better cache usage
COPY go.mod go.sum ./
RUN go mod download

# Then copy the rest of the source
COPY . .

# Build with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o unifi-access-hubitat-middleware ./cmd

# Stage 2: Create minimal runtime image
FROM debian:bookworm-slim

# Create user and group
RUN groupadd -r uahm && useradd -r -g uahm uahm

# Copy binary from builder
COPY --from=builder /app/unifi-access-hubitat-middleware /usr/local/bin/unifi-access-hubitat-middleware

# Expose port
EXPOSE 9423

# Set user for better security
USER uahm

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/unifi-access-hubitat-middleware"]
