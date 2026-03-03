# 构建阶段
FROM golang:1.23.0-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制源代码（包含 go.mod/go.sum 和本地替换的模块）
COPY . .

# 下载依赖
RUN go mod download

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ptool .

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1000 ptool && \
    adduser -D -u 1000 -G ptool ptool

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/ptool /app/ptool

# 创建配置目录
RUN mkdir -p /home/ptool/.config/ptool && \
    chown -R ptool:ptool /home/ptool

# 切换到非 root 用户
USER ptool

# 设置环境变量
ENV HOME=/home/ptool
ENV TZ=Asia/Shanghai

# 设置入口点
ENTRYPOINT ["/app/ptool"]

# 默认命令
CMD ["--help"]
