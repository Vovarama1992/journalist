# --- stage 1: builder ---
FROM golang:1.25-alpine AS builder

WORKDIR /app

# чтобы тянуть модули
RUN apk add --no-cache git

# зависимости
COPY go.mod go.sum ./
RUN go mod download

# код
COPY . .

# сборка бинаря
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/main.go


# --- stage 2: runner ---
FROM alpine:3.19

WORKDIR /app

# нужные утилиты: сертификаты + ffmpeg/ffprobe
RUN apk add --no-cache ca-certificates ffmpeg

# бинарь
COPY --from=builder /app/server /app/server

EXPOSE 8080
ENV PORT=8080

CMD ["/app/server"]