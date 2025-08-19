# Stage 1: Build the frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Stage 2: Build the backend
FROM golang:1.23-alpine AS backend-builder
# Install build dependencies for CGO
RUN apk add --no-cache git build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY backend/ ./backend/
# Copy frontend build artifacts
COPY --from=frontend-builder /app/frontend/dist ./backend/dist
WORKDIR /app
# Build the Go application with CGO enabled
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /infoclash ./backend

# Stage 3: Create the final lightweight image
FROM alpine:latest
# For SSL certificates
RUN apk add --no-cache ca-certificates
WORKDIR /app
# Copy the binary from the backend builder stage
COPY --from=backend-builder /infoclash .
# Expose the default port defined in the environment variables
EXPOSE 8081
# Set the entrypoint
ENTRYPOINT ["/app/infoclash"]