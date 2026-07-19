// 多租户数据隔离中间件
// 强制为所有租户相关查询自动注入 tenant_id 条件，防越权
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"gorm.io/gorm"
)

// TenantScope 多租户作用域
// 在租户/代理角色访问的路由上启用
// 拦截所有 DB 查询，自动注入 tenant_id 条件
func TenantScope(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 1003, "message": "无法识别租户身份"})
			return
		}

		// 为本次请求的 DB 会话添加租户隔离条件
		scopedDB := db.Set("gorm:tenant_id", tenantID)
		c.Set("db", scopedDB)

		// 注入 GORM Scope（用于支持自动注入 tenant_id 到查询条件）
		c.Set("gorm_scope", func() func(*gorm.DB) *gorm.DB {
			return func(q *gorm.DB) *gorm.DB {
				return q.Where("tenant_id = ?", tenantID)
			}
		}())

		c.Next()
	}
}

// CheckResourceOwnership 校验资源是否属于当前租户
// 用于操作前显式校验资源归属，避免越权
func CheckResourceOwnership(db *gorm.DB, tableName string, resourceID, tenantID uint64) (bool, error) {
	var count int64
	err := db.Table(tableName).Where("id = ? AND tenant_id = ?", resourceID, tenantID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CheckCardOwnership 校验卡密归属（示例：带状态返回）
func CheckCardOwnership(db *gorm.DB, cardID, tenantID uint64) (*model.AppCard, error) {
	var card model.AppCard
	err := db.Where("id = ? AND tenant_id = ?", cardID, tenantID).First(&card).Error
	if err != nil {
		return nil, err
	}
	return &card, nil
}
