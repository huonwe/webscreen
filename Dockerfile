# --- 第一阶段：构建二进制文件 ---
FROM golang:1.25.4-alpine AS builder

# 安装构建所需的依赖
RUN apk add --no-cache git

WORKDIR /app

# 先拷贝依赖文件以利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源代码并编译
# 注意：项目使用了 go:embed，所以必须确保 public 文件夹和 scrcpy-server 存在
COPY . .
RUN go build -o webscreen main.go

# --- 第二阶段：运行环境 ---
FROM alpine:latest

# 安装 adb 工具
RUN apk add --no-cache android-tools

WORKDIR /app

# 从构建阶段拷贝编译好的程序
COPY --from=builder /app/webscreen .

ENV PORT=8079

# 暴露 Web 服务端口 (在 webservice/webmaster.go 中定义为 8079)
EXPOSE 8079

# 启动程序
# 提示：在容器内运行 adb 时，通常需要通过环境变量或链接挂载宿主机的 adb server，
# 或者在容器内启动一个新的 adb server。
ENTRYPOINT ["./webscreen", "-port", "${PORT}"]