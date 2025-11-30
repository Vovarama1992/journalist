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
    apt clean

# --- ставим Node.js ---
RUN curl -fsSL https://nodejs.org/dist/v20.11.1/node-v20.11.1-linux-x64.tar.xz -o node.tar.xz && \
    tar -xJf node.tar.xz && \
    mv node-v20.11.1-linux-x64 /usr/local/node && \
    rm node.tar.xz

# критично: прописываем node в PATH (иначе yt-dlp его НЕ видит)
ENV PATH="/usr/local/node/bin:${PATH}"

# проверка что node работает
RUN node -v

# --- устанавливаем yt-dlp ---
RUN pip3 install --break-system-packages yt-dlp

WORKDIR /app
COPY --from=builder /app/server /app/server

EXPOSE 8080

RUN which node && node -v && which yt-dlp && yt-dlp --version

CMD ["/app/server"]