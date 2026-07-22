// Package main 是 KeyAuth SaaS 后端服务入口
// 负责：配置加载、依赖初始化、路由注册、HTTP 启动
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/analysis"
	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/handler"
	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/router"
	"github.com/your-org/keyauth-saas/pkg/snowflake"
)

// @title KeyAuth SaaS API
// @version 0.2.0
// @description 面向开发者的多租户卡密验证平台
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// 1. 解析启动参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 2. 加载配置（铁律 05：所有可变参数走配置）
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] 加载配置失败: %v", err)
	}

	// 2.1 初始化结构化日志（v0.4.0：基于 log/slog，零依赖）
	// 日志级别 / 格式 / 输出路径 优先从 config 读取，可后续扩展为 sys_config 热更新
	logger.Init(logger.Options{
		Level:  cfg.App.LogLevel,
		Format: cfg.App.LogFormat,
		Output: cfg.App.LogOutput,
	})

	// 3. 初始化依赖（数据库、Redis、密钥等）
	container, err := config.InitContainer(cfg)
	if err != nil {
		log.Fatalf("[FATAL] 初始化依赖失败: %v", err)
	}
	defer container.Close()

	// 3.1 v0.5.0 多实例无状态化：通过 Redis 协调分配 snowflake workerID
	// 单实例部署无需此步（默认 workerID=1）；多实例部署时每个实例分配不同 workerID 避免冲突
	// 铁律 06：Redis 不可用时降级为默认 workerID，不阻断启动
	workerID := snowflake.InitWorkerFromRedis(container.Redis)
	log.Printf("[INFO] snowflake workerID = %d", workerID)

	// 4. 注册路由
	engine := router.Register(container)

	// 4.1 启动登录失败日志异步消费 worker（v0.3.1）
	deps := &handler.Deps{
		DB:          container.DB,
		Redis:       container.Redis,
		Crypto:      container.Crypto,
		Config:      container.Config,
		CfgCache:    container.ConfigCache(),
		AnalysisMgr: analysis.NewManager(container.DB, container.ConfigCache()), // v0.6.0 高级分析
	}
	handler.StartLoginFailureWorker(deps)
	handler.StartVerifyLogWorker(deps)
	handler.StartOperationLogWorker(deps)

	// 4.2 v0.6.0 启动高级分析聚合 worker（后台 goroutine）
	// 职责：定时聚合 log_verify → user_behavior_profile + card_usage_profile + 重算 user_risk_score
	// 间隔从 sys_config analysis.aggregate_interval_seconds 读取（默认 3600s）
	// 铁律 06：worker 异常不阻断主流程，ctx 随服务退出取消
	analysisCtx, analysisCancel := context.WithCancel(context.Background())
	defer analysisCancel()
	go analysis.StartAggregationWorker(analysisCtx, deps.AnalysisMgr)

	// 5. 启动 HTTP 服务（v0.6.6：支持端口冲突自动 +1 重试）
	// 端口探测与 serve 解耦：先用 net.Listen 找可用端口，再 srv.Serve(listener)
	// 好处：端口冲突可重试，且 listener 已绑定后 serve 不会再因端口问题失败
	listener, actualPort, err := listenWithPortRetry(cfg.App.Port, cfg.App.PortAutoIncrement, cfg.App.PortMaxAttempts)
	if err != nil {
		log.Fatalf("[FATAL] HTTP 监听失败: %v", err)
	}
	if actualPort != cfg.App.Port {
		log.Printf("[WARN] 原始端口 :%s 被占用，已自动切换到 :%s", cfg.App.Port, actualPort)
	}

	srv := &http.Server{
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("[INFO] KeyAuth SaaS 服务启动，监听 :%s", actualPort)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP 服务异常: %v", err)
		}
	}()

	// 6. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[INFO] 接收到退出信号，开始优雅关闭...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] 服务关闭失败: %v", err)
	}
	log.Println("[INFO] 服务已退出")
}

// listenWithPortRetry 监听 TCP 端口，支持端口冲突自动 +1 重试（v0.6.6 新增）
//
// 设计说明：
//   - 端口探测与 HTTP serve 解耦，先用 net.Listen 找可用端口，再交给 srv.Serve
//   - autoIncrement=false：只尝试一次，失败直接返回（保持 v0.6.5 前的原行为，生产安全）
//   - autoIncrement=true：端口被占用（EADDRINUSE）时 +1 重试，最多 maxAttempts 次
//   - 只对 EADDRINUSE 重试，其他错误（如权限不足、地址格式错误）直接返回，避免无意义重试
//
// 参数：
//   - port: 起始端口（字符串，如 "8080"）
//   - autoIncrement: 端口被占用时是否自动 +1 重试
//   - maxAttempts: 最大尝试次数（含起始端口），autoIncrement=true 时生效，<=0 时取默认 20
//
// 返回：
//   - listener: 已绑定的 net.Listener（调用方负责 Close，通常由 srv.Serve 接管）
//   - actualPort: 实际监听到的端口（字符串，可能与入参 port 不同）
//   - err: 错误
func listenWithPortRetry(port string, autoIncrement bool, maxAttempts int) (net.Listener, string, error) {
	startPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, "", fmt.Errorf("无效的端口号 %q: %w", port, err)
	}
	if startPort < 1 || startPort > 65535 {
		return nil, "", fmt.Errorf("端口号 %d 超出有效范围 1-65535", startPort)
	}

	// autoIncrement=false：单次尝试，失败直接返回（生产模式默认行为）
	if !autoIncrement {
		ln, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return nil, "", err
		}
		return ln, port, nil
	}

	// autoIncrement=true：端口冲突时 +1 重试
	if maxAttempts <= 0 {
		maxAttempts = 20
	}
	for i := 0; i < maxAttempts; i++ {
		curPort := startPort + i
		if curPort > 65535 {
			break
		}
		addr := ":" + strconv.Itoa(curPort)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, strconv.Itoa(curPort), nil
		}
		// 只对端口占用错误重试，其他错误（权限不足等）直接返回
		if !isAddrInUse(err) {
			return nil, "", err
		}
		log.Printf("[WARN] 端口 %s 被占用，尝试下一个端口（第 %d/%d 次）", addr, i+1, maxAttempts)
	}
	return nil, "", fmt.Errorf("端口 %d-%d 全部被占用，已尝试 %d 次", startPort, startPort+maxAttempts-1, maxAttempts)
}

// isAddrInUse 判断错误是否为端口占用（EADDRINUSE）
func isAddrInUse(err error) bool {
	var sysErr *os.SyscallError
	if errors.As(err, &sysErr) {
		return errors.Is(sysErr.Err, syscall.EADDRINUSE)
	}
	return false
}
