FROM golang:1.21-alpine AS builder
# Go version updated to 1.23.0 in go.mod, builder should match or be compatible
# Using golang:1.23-alpine to match the project's Go version
# FROM golang:1.21-alpine AS builder
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
# Optional: Verify dependencies if needed
# RUN go mod verify

# Copy source code
# Ensure .dockerignore is effective to prevent copying unnecessary files
COPY . .

# Build the application
# Using -ldflags="-w -s" to strip debug information and reduce binary size
# CGO_ENABLED=0 is good for static linking
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS communication
RUN apk --no-cache add ca-certificates

# Create a non-root user and group
# Using -D for no password, -H for no home directory (or specify one if needed)
# -S for system user/group
RUN addgroup -S appgroup && adduser -S -D -H appuser -G appgroup

# Set working directory for the final stage
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy environment file template (useful for reference or if app loads it directly)
COPY --from=builder /app/.env.example .

# Ensure the binary is executable by the user.
# Alpine's adduser usually sets up reasonable permissions, and Go binaries are typically 755.
# If WORKDIR /app was owned by root, chown might be needed for .env.example if appuser needs to modify/replace it at runtime.
# For a statically copied binary and .env.example, appuser just needs read/execute.
# RUN chown appuser:appgroup main .env.example # Usually not needed if permissions are fine

# Expose port (ensure this port is > 1024 for non-root user or handle permissions)
EXPOSE 8080

# Set the user
USER appuser

# Run the application
CMD ["./main"]
