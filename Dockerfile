# 1. Используем официальный базовый образ для Go
# FROM golang:1.23 AS builder
FROM golang:alpine AS builder

# 2. Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

RUN apk update && apk upgrade
RUN apk add --no-cache gcc musl-dev
RUN apk add --no-cache sqlite sqlite-dev

# 3. Копируем модули Go (go.mod и go.sum) и загружаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# 4. Копируем все файлы проекта
COPY . .

# 5. Сборка приложения
ENV CGO_ENABLED=1
RUN go build -o main .

# 6. Создаем минимальный образ для выполнения
FROM alpine:latest

# 7. Устанавливаем зависимости, необходимые для запуска SQLite
RUN apk update && apk upgrade
RUN apk add --no-cache sqlite

# 8. Устанавливаем рабочую директорию для запуска приложения
WORKDIR /app

# 9. Копируем скомпилированное приложение из стадии сборки
COPY --from=builder /app/main .

# 10. Копируем базу данных (если требуется)
# COPY currencies.db .

# 11. Устанавливаем команду по умолчанию для запуска
CMD ["./main"]
