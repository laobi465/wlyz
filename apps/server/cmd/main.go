// Package main 是 KeyAuth SaaS 后端服务入口
// 负责：配置加载、依赖初始化、路由注册、HTTP 启动
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/handler"
	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/router"
	"github.com/your-org/keyauth-saas/apps/server/pkg/snowflake"
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
		DB:       container.DB,
		Redis:    container.Redis,
		Crypto:   container.Crypto,
		Config:   container.Config,
		CfgCache: container.ConfigCache(),
	}
	handler.StartLoginFailureWorker(deps)
	handler.StartVerifyLogWorker(deps)
	handler.StartOperationLogWorker(deps)

	// 5. 启动 HTTP 服务
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("[INFO] KeyAuth SaaS 服务启动，监听 :%s", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
