FROM golang:alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Add build arguments for service configuration
ARG SERVICE_NAME
ARG SERVICE_PORT

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/app ./cmd/${SERVICE_NAME}

# Use a small alpine image for the final image
FROM alpine:3.19

# Install necessary runtime packages
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user to run the application
RUN adduser -D -g '' appuser

# Add build argument for environment
ARG ENVIRONMENT=development
ARG SERVICE_PORT

# Copy the binary and appropriate .env from the builder stage
COPY --from=builder /go/bin/app /app
COPY --from=builder /app/docker/env/${ENVIRONMENT}.env /.env

# Set ownership of the application binary and .env
RUN chown appuser:appuser /app /.env

# Use the non-root user
USER appuser

# Expose the port the service runs on
EXPOSE ${SERVICE_PORT}

# Command to run the application
CMD ["/app"] 