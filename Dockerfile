FROM python:3.13-slim AS ytdlp

RUN apt update && \
    apt install -y ffmpeg ca-certificates curl && \
    pip install --break-system-packages yt-dlp

FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/journalist

FROM debian:bookworm-slim
WORKDIR /app

RUN apt update && apt install -y ffmpeg ca-certificates python3 && apt clean

COPY --from=ytdlp /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp
COPY --from=builder /app/app /app/app

EXPOSE 8080
CMD ["/app/app"]