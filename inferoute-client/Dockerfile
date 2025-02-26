
# Stage 1: Build the provider client binary
FROM golang:1.20-alpine AS builder

# Install dependencies needed for building
RUN apk add --no-cache git

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary (assuming your main.go is in cmd/main.go)
RUN CGO_ENABLED=0 GOOS=linux go build -o provider-client ./cmd/main.go

# Stage 2: Create a small runtime image including cloudflared
FROM alpine:latest

# Install cloudflared from the official package (update URL if necessary)
RUN apk add --no-cache ca-certificates && \
    wget -q -O /tmp/cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb && \
    apk add --no-cache dpkg && \
    dpkg -i /tmp/cloudflared.deb && \
    rm /tmp/cloudflared.deb && \
    rm -rf /var/cache/apk/*

# Copy the binary from the builder stage
COPY --from=builder /app/provider-client /provider-client

# Expose necessary ports (example: 8080 for the provider client API)
EXPOSE 8080

# Set environment variables if needed (can also be provided in docker-compose)
ENV ENV=production

# Start the provider client along with cloudflared tunnel in the background
CMD ["/bin/sh", "-c", "cloudflared tunnel --no-autoupdate run & /provider-client"]