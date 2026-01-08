# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o backendAPI .

# Stage 2: Production runner
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy binary from builder
COPY --from=builder /app/backendAPI .

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./backendAPI"]
