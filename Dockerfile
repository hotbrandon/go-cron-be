# ----------- Build stage -----------
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /app

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the Go application.
# CGO is disabled as the MySQL driver is pure Go.
# ldflags are used to create a smaller binary.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o main .



# ----------- Runtime stage -----------
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /app

# Copy the built binary from the builder
COPY --from=builder /app/main .

# Create non-root user for security
RUN addgroup -g 1001 appgroup && \
    adduser -D -s /bin/sh -u 1001 -G appgroup appuser

# Set ownership of the binary and switch user
RUN chown appuser:appgroup /app/main
USER appuser

# Expose port (optional for HTTP)
EXPOSE 8005

# Health check (optional)
HEALTHCHECK --interval=60s --timeout=10s --start-period=10s --retries=3 \
    CMD pgrep -f main || exit 1

# Run the app
CMD ["./main"]
