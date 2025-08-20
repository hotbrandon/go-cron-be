# ----------- Build stage -----------
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGO + SQLite
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev \
    sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the Go application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .



# ----------- Runtime stage -----------
FROM alpine:latest

# Install runtime dependencies (if needed)
RUN apk --no-cache add \
    ca-certificates \
    tzdata

# Set working directory and create data dir
WORKDIR /app
RUN mkdir -p /app/data

# Copy the built binary from the builder
COPY --from=builder /app/main .

# Create non-root user for security
RUN addgroup -g 1001 appgroup && \
    adduser -D -s /bin/sh -u 1001 -G appgroup appuser

# Set permissions and switch user
RUN chown -R appuser:appgroup /app
USER appuser

# Expose port (optional for HTTP)
EXPOSE 8005

# Health check (optional)
HEALTHCHECK --interval=60s --timeout=10s --start-period=10s --retries=3 \
    CMD pgrep -f main || exit 1

# Run the app
CMD ["./main"]
