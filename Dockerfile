# --- stage 1: builder ---
FROM golang:1.25 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/main.go


# --- stage 2: runner ---
FROM debian:stable-slim

RUN apt update && \
    apt install -y curl ffmpeg ca-certificates python3 python3-pip xz-utils && \
    pip3 install --break-system-packages yt-dlp && \
    apt clean

# --- ставим Node.js вручную, гарантированно рабочий ---
RUN curl -fsSL https://nodejs.org/dist/v20.11.1/node-v20.11.1-linux-x64.tar.xz -o node.tar.xz && \
    tar -xJf node.tar.xz && \
    mv node-v20.11.1-linux-x64 /usr/local/node && \
    ln -s /usr/local/node/bin/node /usr/local/bin/node && \
    ln -s /usr/local/node/bin/npm /usr/local/bin/npm && \
    rm node.tar.xz

# теперь node точно в PATH и yt-dlp его увидит
RUN node -v

WORKDIR /app
COPY --from=builder /app/server /app/server

EXPOSE 8080
CMD ["/app/server"]