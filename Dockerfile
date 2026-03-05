# ─── Stage 1: Builder ─────────────────────────────────────────────────────────
# Uses the full Go image to compile the binary.
# This stage is NOT included in the final image — only the binary is copied.
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go.mod and go.sum first — Docker caches this layer.
# If your source files change but go.mod doesn't, Docker reuses this layer
# and skips re-downloading dependencies (faster builds).
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the binary
# CGO_ENABLED=0: static binary (no C dependencies) — required for scratch image
# -ldflags "-s -w": strip debug symbols to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /bin/eform-backend ./cmd/server/main.go


# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
# Uses a minimal Alpine image — no build tools, no shell (security best practice).
# The final image is ~15MB vs ~800MB for a full Go image.
FROM alpine:3.20

# Install only what we need at runtime:
# - ca-certificates: needed for HTTPS calls to VIDA API
# - tzdata: needed for correct timezone handling (WIB = Asia/Jakarta)
RUN apk add --no-cache ca-certificates tzdata

# Set timezone to WIB (Waktu Indonesia Barat) — Jakarta
ENV TZ=Asia/Jakarta

# Create a non-root user for security
# Running as root in a container is a security risk
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -s /bin/sh -D appuser

# Create storage directory with correct permissions
# This is for local file storage (KTP images, contracts, etc.)
RUN mkdir -p /var/app/storage && chown appuser:appgroup /var/app/storage

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /bin/eform-backend .

# Copy keys directory (should be mounted as a volume in production,
# but included here for reference)
# COPY keys/ ./keys/

# Switch to non-root user
USER appuser

# Expose the application port
EXPOSE 8080

# Health check — Docker will restart the container if this fails
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# Start the server
CMD ["./eform-backend"]