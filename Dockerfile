# --- stage 1: build ---
FROM golang:1.25-alpine AS builder

WORKDIR /app

# ускоряем сборку
RUN apk add --no-cache git

# зависимости
COPY go.mod go.sum ./
RUN go mod download

# весь код
COPY . .

# собираем бинарь
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/main.go


# --- stage 2: runner ---
FROM alpine:3.19

WORKDIR /app

# чтобы не было ошибок DNS/SSL
RUN apk add --no-cache ca-certificates

# копируем бинарь
COPY --from=builder /app/server /app/server

EXPOSE 8080
ENV PORT=8080

CMD ["/app/server"]