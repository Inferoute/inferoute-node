# Use specific version instead of latest
FROM alpine:3.19

# Create non-root user first for better layer caching
RUN addgroup -g 1001 -S tunneluser && \
    adduser -u 1001 -S tunneluser -G tunneluser

# Install necessary packages in a single layer
RUN apk add --no-cache \
    curl \
    jq \
    bash \
    coreutils \
    netcat-openbsd && \
    # Install cloudflared from official source
    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /usr/local/bin/cloudflared && \
    chmod +x /usr/local/bin/cloudflared

# Create working directory with proper ownership
WORKDIR /app
RUN chown tunneluser:tunneluser /app

# Copy our runner script
COPY --chown=tunneluser:tunneluser run_tunnel.sh debug_tunnel.sh /app/

# Make script executable
RUN chmod +x /app/run_tunnel.sh /app/debug_tunnel.sh

# Switch to non-root user
USER tunneluser

# Add health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep cloudflared || exit 1

# Set default command
CMD ["/app/run_tunnel.sh"] 