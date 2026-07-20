// Package handler HTTP 处理器层
// 仅负责参数校验、调用 Service、封装响应
package handler

import (
	"github.com/redis/go-redis/v9"
	"github.com/your-org/keyauth-saas/apps/server/internal/analysis"
	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/risk"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"gorm.io/gorm"
)

// Deps 处理器依赖（统一注入，避免全局变量）
type Deps struct {
	DB          *gorm.DB
	Redis       *redis.Client
	Crypto      *crypto.Manager
	Config      *config.Config
	CfgCache    *config.ConfigCache
	RiskMgr     *risk.Manager     // v0.4.0 风控规则引擎（nil=禁用风控）
	AnalysisMgr *analysis.Manager // v0.6.0 高级分析（nil=禁用分析）
}
