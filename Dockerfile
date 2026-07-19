# ============================================================
# KeyAuth SaaS Dockerfile —— 后端 Go 服务
# 多阶段构建：builder -> runtime
# ============================================================

# ---------- Stage 1: builder ----------
FROM golang:1.22-alpine AS builder

ARG APP_VERSION=0.2.0
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct

WORKDIR /build

# 利用缓存：先复制依赖文件
COPY apps/server/go.mod apps/server/go.sum* ./apps/server/
RUN cd apps/server && go mod download

# 复制源码
COPY apps/server ./apps/server

# 编译：静态二进制
RUN cd apps/server && go build -trimpath -ldflags="-s -w -X main.Version=${APP_VERSION}" -o /out/keyauth-server ./cmd

# ---------- Stage 2: runtime ----------
FROM alpine:3.19

ARG APP_VERSION=0.2.0
ENV APP_VERSION=${APP_VERSION} \
    TZ=Asia/Shanghai

# 安装最小运行时依赖 + 时区
RUN apk add --no-cache ca-certificates tzdata wget \
    && cp /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo "${TZ}" > /etc/timezone

WORKDIR /app

# 复制二进制
COPY --from=builder /out/keyauth-server /app/keyauth-server

# 复制 migrations（用于启动时自动迁移）
COPY apps/server/migrations /app/migrations

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

EXPOSE 8080

ENTRYPOINT ["/app/keyauth-server"]
CMD ["--config=/app/configs/config.yaml"]
