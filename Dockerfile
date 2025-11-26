# --- stage 1: builder ---
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/journalist

# --- stage 2: runner ---
FROM debian:bookworm-slim

RUN apt update && \
    apt install -y ffmpeg ca-certificates python3 python3-pip && \
    apt clean

RUN pip3 install yt-dlp

WORKDIR /app
COPY --from=builder /app/app /app/app

EXPOSE 8080
ENV PORT=8080

CMD ["/app/app"]