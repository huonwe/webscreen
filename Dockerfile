# --- 第一阶段：构建二进制文件 ---
FROM golang:1.25.4-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o webscreen main.go

FROM alpine:latest

RUN apk add --no-cache android-tools

WORKDIR /app

COPY --from=builder /app/webscreen .

ENV PORT=8079

EXPOSE 8079

ENTRYPOINT ./webscreen -port $PORT
