# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o syncer-bin .

# Stage 2: Production runner
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS connections and wget for healthcheck
RUN apk --no-cache add ca-certificates wget

# Create non-root user with static UID to match K8s runAsUser
RUN addgroup -S -g 1000 appgroup && adduser -S -u 1000 -G appgroup appuser

# Copy binary from builder
COPY --from=builder /build/syncer-bin ./syncer

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Run the application
CMD ["./syncer"]
