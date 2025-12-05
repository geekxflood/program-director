# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /app/program-director .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 program-director

# Copy binary from builder
COPY --from=builder /app/program-director /app/program-director

# Create directories for config and data
RUN mkdir -p /app/config /app/data && \
    chown -R program-director:program-director /app

# Switch to non-root user
USER program-director

# Default configuration path
ENV CONFIG_PATH=/app/config/config.yaml

# Expose HTTP port for server mode
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/program-director", "version"]

# Default command
ENTRYPOINT ["/app/program-director"]
CMD ["generate", "--all-themes"]
