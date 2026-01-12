# Stage 1: Build
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build ทั้ง API และ Worker
RUN go build -o main-api cmd/api/main.go
RUN go build -o main-worker cmd/worker/main.go

# Stage 2: Run (ใช้ Image เล็กๆ เพื่อความเบา)
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main-api .
COPY --from=builder /app/main-worker .
COPY index.html .

# Default command (เดี๋ยวเราจะ Override ใน docker-compose)
CMD ["./main-api"]