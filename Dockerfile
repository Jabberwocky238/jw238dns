# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o jw238dns \
    ./cmd/jw238dns

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 jw238dns && \
    adduser -D -u 1000 -G jw238dns jw238dns

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/jw238dns /app/jw238dns

# Copy assets (GeoIP databases, examples)
COPY --from=builder /build/assets /app/assets

# Create directories for data
RUN mkdir -p /app/data /app/certs && \
    chown -R jw238dns:jw238dns /app

# Switch to non-root user
USER jw238dns

# Expose DNS ports (UDP and TCP)
EXPOSE 53/udp 53/tcp

# Expose HTTP management port
EXPOSE 8080/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/jw238dns", "healthcheck"]

# Run the application
ENTRYPOINT ["/app/jw238dns"]
CMD ["serve"]
