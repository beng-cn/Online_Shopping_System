# ============================================
# 阶段一：编译阶段（使用完整 Go 镜像）
# ============================================
FROM golang:1.25-alpine AS builder

# 设置 Go 编译参数
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY=https://goproxy.cn,direct

WORKDIR /app

# 先复制依赖文件，利用 Docker 层缓存——依赖不变则跳过下载
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译（静态链接，不依赖 libc）
COPY . .
RUN go build -ldflags="-s -w" -o /app/server ./cmd/server/

# ============================================
# 阶段二：运行阶段（使用最小 scratch 镜像）
# ============================================
FROM alpine:3.21

# 最小运行时依赖
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从编译阶段复制产物
COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs

# 创建上传目录
RUN mkdir -p /app/uploads

EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./server"]
