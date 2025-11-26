# --- stage 1: builder ---
FROM golang:1.25 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/main.go


# --- stage 2: runner ---
FROM debian:stable-slim

# Устанавливаем ffmpeg + python + pip + nodejs (JS runtime)
RUN apt update && \
    apt install -y ffmpeg ca-certificates python3 python3-pip nodejs npm && \
    pip3 install --break-system-packages yt-dlp && \
    apt clean

WORKDIR /app
COPY --from=builder /app/server /app/server

EXPOSE 8080
CMD ["/app/server"]