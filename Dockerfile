# --- Stage 1: build the Svelte frontend ---
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build
# The build output will be in /app/frontend/dist

# --- Stage 2: build the Go backend ---
FROM golang:1.22-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
# Build statically so the final image can be minimal
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# --- Stage 3: final minimal image ---
FROM alpine:3.20
WORKDIR /app

# TLS certificates required so the Go backend can call Google's API over HTTPS
RUN apk add --no-cache ca-certificates

COPY --from=backend-build /app/backend/server ./server
COPY --from=frontend-build /app/frontend/dist ./static

EXPOSE 8080

CMD ["./server"]
