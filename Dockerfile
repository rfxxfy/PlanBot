# Multi-stage build for optimal image size
FROM golang:1.21-alpine AS builder

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
    -ldflags="-w -s -X main.Version=${VERSION:-dev}" \
    -o planbot \
    main.go

# Final stage - minimal image
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata postgresql-client

# Create non-root user
RUN addgroup -g 1000 planbot && \
    adduser -D -u 1000 -G planbot planbot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/planbot .
COPY --from=builder /build/database/*.sql ./database/

# Set ownership
RUN chown -R planbot:planbot /app

# Switch to non-root user
USER planbot

# Set timezone (can be overridden)
ENV TZ=Europe/Moscow

# Healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Expose healthcheck port
EXPOSE 8080

# Run the application
CMD ["./planbot"]

