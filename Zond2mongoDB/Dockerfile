# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy everything first to ensure module context is preserved
COPY . .

# Download dependencies and build
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o synchroniser .

# Stage 2: Production runner
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy binary from builder
COPY --from=builder /build/synchroniser .

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Run the application
CMD ["./synchroniser"]
