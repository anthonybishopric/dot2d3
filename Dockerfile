# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /dot2d3 ./cmd/dot2d3

# Runtime stage
FROM alpine:3.19

# Add ca-certificates for HTTPS (if needed) and create non-root user
RUN apk --no-cache add ca-certificates && \
    adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /dot2d3 /app/dot2d3

# Use non-root user
USER appuser

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the server
ENTRYPOINT ["/app/dot2d3"]
CMD ["-serve", ":8080"]
