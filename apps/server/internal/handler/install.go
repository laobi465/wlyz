// 安装向导 Handler（v0.3.6）
// 首次部署时通过 Web 表单配置超管账号 + 平台基础参数，替代原 seed 占位 hash + 后置脚本方案
// 严格遵循铁律 04/05/06：
//   04 - 不硬编码任何密钥/密码，全部由安装表单传入
//   05 - 安装写入的参数走 sys_config 表 + Redis 缓存
//   06 - 已安装状态检测用 sys_admin 表 hash 是否为占位串，禁用占位 hash 检测可能误判
package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"gorm.io/gorm"
)

// installPlaceholderHash seed 002 中的占位 bcrypt hash 标记前缀
// 用于判断 sys_admin 是否仍为"未安装"状态
const installPlaceholderHash = "PLACEHOLDER_BCRYPT_HASH"

// ============== 安装状态检测 ==============

// InstallStatus GET /api/v1/install/status
// 返回 installed bool + 当前配置概览
func InstallStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		installed, err := checkInstalled(deps.DB)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "检测安装状态失败: "+err.Error())
			return
		}

		// 已安装时返回脱敏概览；未安装时返回空
		var adminName string
		var domain string
		if installed {
			var admin model.SysAdmin
			if err := deps.DB.First(&admin, 1).Error; err == nil {
				adminName = admin.Username
			}
			domain = deps.CfgCache.GetString(c.Request.Context(), "platform.domain", "")
		}

		middleware.Success(c, gin.H{
			"installed":   installed,
			"admin_name":  adminName,
			"domain":      domain,
			"server_time": time.Now().Format(time.RFC3339),
		})
	}
}

// checkInstalled 通过 sys_admin.hash 是否为占位串判定
// 铁律 06：不能用 count(*) > 0 判定（seed 已插入 1 行占位）
func checkInstalled(db *gorm.DB) (bool, error) {
	var admin model.SysAdmin
	if err := db.First(&admin, 1).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	// 占位 hash 标记 → 未安装；真实 bcrypt hash（$2a$...）→ 已安装
	return !strings.Contains(admin.PasswordHash, installPlaceholderHash), nil
}

// ============== 执行安装 ==============

// installReq 安装请求体
type installReq struct {
	// 步骤 2：超管账号
	AdminUsername string `json:"admin_username" binding:"required,min=3,max=64"`
	AdminPassword string `json:"admin_password" binding:"required,min=8,max=64"`
	AdminEmail    string `json:"admin_email" binding:"omitempty,email,max=128"`
	AdminPhone    string `json:"admin_phone" binding:"omitempty,max=32"`

	// 步骤 3：平台基础配置（全部写入 sys_config）
	PlatformDomain     string `json:"platform_domain" binding:"omitempty,max=255"`      // 平台主域名
	PlatformName       string `json:"platform_name" binding:"omitempty,max=128"`        // 平台名称
	NotifyEmail        string `json:"notify_email" binding:"omitempty,email,max=128"`   // 系统通知邮箱
	AgentRegisterFee   string `json:"agent_register_fee" binding:"omitempty,max=10"`    // 代理注册费（元）
	PlatformCommission string `json:"platform_commission" binding:"omitempty,max=10"`   // 平台抽成比例（0-1）
}

// Install POST /api/v1/install
// 首次部署执行：写入超管真实密码 hash + 平台基础配置
func Install(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 二次校验：已安装则拒绝
		installed, err := checkInstalled(deps.DB)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "检测安装状态失败: "+err.Error())
			return
		}
		if installed {
			middleware.Fail(c, http.StatusForbidden, 1003, "系统已安装，如需重新配置请联系管理员")
			return
		}

		// 2. 参数校验
		var req installReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 3. 计算 bcrypt 哈希（cost=12）
		hash, err := crypto.HashPassword(req.AdminPassword)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "密码加密失败: "+err.Error())
			return
		}

		// 4. 事务写入
		ctx := c.Request.Context()
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 4.1 更新 sys_admin（id=1 的占位行 → 真实账号）
			updates := map[string]interface{}{
				"username":      req.AdminUsername,
				"password_hash": hash,
				"email":         req.AdminEmail,
				"phone":         req.AdminPhone,
				"status":        "active",
				"totp_secret":   nil,
				"last_login_at": nil,
				"last_login_ip": "",
			}
			if err := tx.Model(&model.SysAdmin{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
				return err
			}

			// 4.2 写入平台基础配置到 sys_config（upsert）
		// 铁律 05：所有可变参数走 sys_config，不写入代码或 .env
		// 铁律 06：键名与 migrations/002_seed_data.up.sql 保持一致（点号分隔，不用下划线）
		configs := map[string]string{
			"platform.domain":                req.PlatformDomain,
			"platform.name":                  req.PlatformName,
			"platform.notify_email":          req.NotifyEmail,
			"agent.register.fee":             req.AgentRegisterFee,
			"pay.platform.commission_rate":   req.PlatformCommission,
			"platform.installed_at":          time.Now().Format(time.RFC3339),
		}
			for key, value := range configs {
				if value == "" {
					continue // 空值跳过，保留 sys_config 已有默认值
				}
				if err := upsertConfig(tx, key, value); err != nil {
					return err
				}
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "安装失败: "+txErr.Error())
			return
		}

		// 5. 刷新 sys_config Redis 缓存（下次读取自动从 DB 重建）
		_ = deps.CfgCache.InvalidateAll(ctx)

		// 6. 异步记录操作日志（不含密码）
		RecordOperation(deps, c, "install", "system_install", "success",
			"platform", nil, map[string]interface{}{
				"admin_username": req.AdminUsername,
				"admin_email":    req.AdminEmail,
				"domain":         req.PlatformDomain,
			})

		middleware.Success(c, gin.H{
			"installed":    true,
			"admin_name":   req.AdminUsername,
			"installed_at": time.Now().Format(time.RFC3339),
			"message":      "安装完成，请使用刚配置的账号登录",
		})
	}
}

// upsertConfig 写入或更新 sys_config 单条记录
// 铁律 05：所有可变参数走 sys_config 表
func upsertConfig(tx *gorm.DB, key, value string) error {
	var existing model.SysConfig
	result := tx.Where("config_key = ?", key).First(&existing)
	if result.Error == gorm.ErrRecordNotFound {
		// 新增
		cfg := model.SysConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ConfigType:  "string",
			ConfigName:  key,
			ConfigGroup: "platform",
			Remark:      "Installed via /install wizard",
		}
		return tx.Create(&cfg).Error
	}
	if result.Error != nil {
		return result.Error
	}
	// 更新
	return tx.Model(&existing).Update("config_value", value).Error
}
