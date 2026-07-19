// Package quota 套餐配额检查 helper
// 用途：统一封装开发者套餐资源上限校验（MaxApps/MaxCards/MaxAgents）
// 严格遵循铁律 04/05/06：不硬编码配额，从 sys_package 表读取
//
// 设计原则：
//  1. 提供原子化的 CheckMax* 函数，各 handler 在写操作前调用
//  2. 不直接返回 HTTP 响应，仅返回 error，由调用方决定如何响应
//  3. error 中含可读的中文提示，可直接作为用户可见错误
//  4. 不在 helper 内开事务（调用方负责事务边界，避免嵌套事务）
//
// 注意：配额检查存在 TOCTOU 风险（check 后到实际入库前的并发写入）
//   - 严格场景应在事务内 SELECT COUNT + INSERT 一起做（已有 handler 用 FOR UPDATE）
//   - 配额 helper 仅作为「快速拒绝明显超额请求」的第一道防线
package quota

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 自定义错误类型 ==============

// ExceededError 配额超限错误
type ExceededError struct {
	Resource string // 资源类型：apps / cards / agents
	Current  int    // 当前数量
	Limit    int    // 套餐上限
	AddCount int    // 本次新增数量（cards 场景）
}

func (e *ExceededError) Error() string {
	if e.AddCount > 0 {
		return fmt.Sprintf("将超过套餐%s上限 %d（当前 %d，本次新增 %d）",
			e.Resource, e.Limit, e.Current, e.AddCount)
	}
	return fmt.Sprintf("已达套餐%s上限 %d（当前 %d）", e.Resource, e.Limit, e.Current)
}

// Is 适配 errors.Is，便于调用方用 errors.Is(err, &quota.ExceededError{}) 判断
func (e *ExceededError) Is(target error) bool {
	_, ok := target.(*ExceededError)
	return ok
}

// ============== 内部辅助 ==============

// loadTenantPackage 加载开发者 + 套餐
// 返回 (tenant, package, error)；如果套餐不存在返回错误
func loadTenantPackage(db *gorm.DB, tenantID uint64) (model.SysTenant, model.SysPackage, error) {
	var tenant model.SysTenant
	if err := db.Select("id, status, package_id, expires_at").First(&tenant, tenantID).Error; err != nil {
		return tenant, model.SysPackage{}, fmt.Errorf("查询开发者失败: %w", err)
	}
	if tenant.Status != "active" {
		return tenant, model.SysPackage{}, errors.New("开发者账号已被禁用或待审核")
	}
	if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
		return tenant, model.SysPackage{}, errors.New("开发者套餐已过期，请续费")
	}

	var pkg model.SysPackage
	if err := db.First(&pkg, tenant.PackageID).Error; err != nil {
		return tenant, pkg, fmt.Errorf("查询套餐失败: %w", err)
	}
	if pkg.Status != "active" {
		return tenant, pkg, errors.New("套餐已被禁用")
	}
	return tenant, pkg, nil
}

// ============== 公开 Check 函数 ==============

// CheckMaxApps 校验开发者创建应用是否超出套餐上限
// 调用时机：TenantCreateApp 入库前
// 入参：
//   - db: gorm.DB 实例
//   - tenantID: 开发者 ID
//
// 返回：超限时返回 *ExceededError（Resource="应用数"），其他错误为系统错误
func CheckMaxApps(db *gorm.DB, tenantID uint64) error {
	_, pkg, err := loadTenantPackage(db, tenantID)
	if err != nil {
		return err
	}
	if pkg.MaxApps <= 0 {
		// 0 表示不限
		return nil
	}

	var count int64
	if err := db.Model(&model.App{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询应用数失败: %w", err)
	}
	if int(count) >= pkg.MaxApps {
		return &ExceededError{
			Resource: "应用数",
			Current:  int(count),
			Limit:    pkg.MaxApps,
		}
	}
	return nil
}

// CheckMaxCards 校验开发者生成卡密是否超出套餐上限
// 调用时机：TenantGenerateCards 入库前
// 入参：
//   - db: gorm.DB 实例
//   - tenantID: 开发者 ID
//   - addCount: 本次计划生成的卡密数量
//
// 返回：超限时返回 *ExceededError（Resource="卡密数"），其他错误为系统错误
func CheckMaxCards(db *gorm.DB, tenantID uint64, addCount int) error {
	_, pkg, err := loadTenantPackage(db, tenantID)
	if err != nil {
		return err
	}
	if pkg.MaxCards <= 0 {
		return nil
	}

	var count int64
	if err := db.Model(&model.AppCard{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询卡密数失败: %w", err)
	}
	if int(count)+addCount > pkg.MaxCards {
		return &ExceededError{
			Resource: "卡密数",
			Current:  int(count),
			Limit:    pkg.MaxCards,
			AddCount: addCount,
		}
	}
	return nil
}

// CheckMaxAgents 校验开发者代理数是否超出套餐上限
// 调用时机：
//   - TenantGenInviteCode 邀请码生成前（隐含招募代理意图）
//   - AgentRegister 代理注册前（实际写入代理表前）
//
// 入参：
//   - db: gorm.DB 实例
//   - tenantID: 开发者 ID
//
// 返回：超限时返回 *ExceededError（Resource="代理数"），其他错误为系统错误
func CheckMaxAgents(db *gorm.DB, tenantID uint64) error {
	_, pkg, err := loadTenantPackage(db, tenantID)
	if err != nil {
		return err
	}
	if pkg.MaxAgents <= 0 {
		// 0 表示该套餐不允许招募代理
		return &ExceededError{
			Resource: "代理数",
			Current:  0,
			Limit:    0,
		}
	}

	var count int64
	if err := db.Model(&model.Agent{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询代理数失败: %w", err)
	}
	if int(count) >= pkg.MaxAgents {
		return &ExceededError{
			Resource: "代理数",
			Current:  int(count),
			Limit:    pkg.MaxAgents,
		}
	}
	return nil
}

// CheckMaxDevices 校验单卡密绑定设备数是否超出应用配置上限
// 调用时机：ClientBind / ClientLogin（自动绑定）入库前
// 入参：
//   - db: gorm.DB 实例
//   - cardID: 卡密 ID
//   - maxDevices: 应用的 MaxDevices 配置
//
// 返回：超限时返回 *ExceededError（Resource="设备数"），其他错误为系统错误
// 注：MaxDevices 是应用级配置（App.MaxDevices），非套餐级
func CheckMaxDevices(db *gorm.DB, cardID uint64, maxDevices int) error {
	if maxDevices <= 0 {
		return nil
	}
	var count int64
	if err := db.Model(&model.AppDevice{}).
		Where("card_id = ? AND status = ?", cardID, "active").
		Count(&count).Error; err != nil {
		return fmt.Errorf("查询设备数失败: %w", err)
	}
	if int(count) >= maxDevices {
		return &ExceededError{
			Resource: "设备数",
			Current:  int(count),
			Limit:    maxDevices,
		}
	}
	return nil
}
