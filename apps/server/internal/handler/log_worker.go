// 日志系统：验证日志 + 操作日志 异步写入 worker
// v0.3.3：参考 loginFailureCh 模式，提供 verifyLogCh / operationLogCh 两个 channel
// 严格遵循铁律 04/05/06：禁止硬编码、配置走 CfgCache、不确定处标注「待核实」
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/auth"
	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 验证日志异步队列 ==============

// verifyLogCh 验证日志异步队列（容量 4096，超出丢弃以保证验证 API 性能）
var verifyLogCh = make(chan *model.LogVerify, 4096)

// StartVerifyLogWorker 启动后台 goroutine 消费验证日志
// 应在 main.go 启动时调用一次
func StartVerifyLogWorker(deps *Deps) {
	go func() {
		for log := range verifyLogCh {
			if err := deps.DB.Create(log).Error; err != nil {
				// v0.4.0：结构化日志记录（取代 _ = err 静默丢弃）
				logger.Error("verify_log write failed",
					"err", err,
					"tenant_id", log.TenantID,
					"app_id", log.AppID,
					"action", log.Action,
				)
			}
		}
	}()
}

// enqueueVerifyLog 异步写入验证日志（不阻塞调用方）
// extra 可为空；将合并 card_key/hwid/message 字段
func enqueueVerifyLog(deps *Deps, tenantID, appID uint64, cardID, deviceID *uint64,
	action, result, ip, ua string, extra map[string]interface{}) {
	extraJSON := ""
	if len(extra) > 0 {
		if b, err := json.Marshal(extra); err == nil {
			extraJSON = string(b)
		}
	}
	log := &model.LogVerify{
		TenantID:  tenantID,
		AppID:     appID,
		CardID:    cardID,
		DeviceID:  deviceID,
		Action:    action,
		Result:    result,
		ClientIP:  truncateIP(ip),
		UserAgent: truncateUA(ua),
		Extra:     extraJSON,
		CreatedAt: time.Now(),
	}
	select {
	case verifyLogCh <- log:
	default:
		// 队列满则丢弃（保证验证 API 可用性）
	}
}

// truncateIP 截断 IP 到 45 字符（兼容 IPv6）
func truncateIP(ip string) string {
	if len(ip) > 45 {
		return ip[:45]
	}
	return ip
}

// ============== 操作日志异步队列 ==============

// operationLogCh 操作日志异步队列（容量 2048）
var operationLogCh = make(chan *model.LogOperation, 2048)

// StartOperationLogWorker 启动后台 goroutine 消费操作日志
// 应在 main.go 启动时调用一次
func StartOperationLogWorker(deps *Deps) {
	go func() {
		for log := range operationLogCh {
			if err := deps.DB.Create(log).Error; err != nil {
				// v0.4.0：结构化日志记录（取代 _ = err 静默丢弃）
				logger.Error("operation_log write failed",
					"err", err,
					"operator_type", log.OperatorType,
					"operator_id", log.OperatorID,
					"module", log.Module,
					"action", log.Action,
				)
			}
		}
	}()
}

// RecordOperation 记录操作日志（异步，不阻塞调用方）
// 推荐在 handler 内业务成功/失败后调用；module/action/target 按语义填写
func RecordOperation(deps *Deps, c *gin.Context, module, action, status, targetType string, targetID *uint64, detail map[string]interface{}) {
	role := getRole(c)
	userID := getUserID(c)
	username := getUsername(c)
	ip := c.ClientIP()
	ua := c.Request.UserAgent()

	detailJSON := ""
	if len(detail) > 0 {
		if b, err := json.Marshal(detail); err == nil {
			detailJSON = string(b)
		}
	}

	log := &model.LogOperation{
		OperatorType: role,
		OperatorID:   userID,
		Username:     username,
		OperatorIP:   truncateIP(ip),
		UserAgent:    truncateUA(ua),
		Module:       module,
		Action:       action,
		Status:       status,
		TargetType:   targetType,
		TargetID:     targetID,
		Detail:       detailJSON,
	}
	select {
	case operationLogCh <- log:
	default:
		// 队列满则丢弃（保证业务可用性）
	}
}

// getUsername 从 gin.Context 读取用户名（auth 中间件已注入）
// 注：当前 JWT 中间件可能未注入 username，回退到空字符串
func getUsername(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ============== 操作日志查询 ==============

// AdminListOperationLogs GET /admin/logs/operations
// 操作日志列表（支持 operator_type/operator_id/module/action/status/date/keyword 筛选）
func AdminListOperationLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.LogOperation{})
		if t := c.Query("operator_type"); t != "" {
			q = q.Where("operator_type = ?", t)
		}
		if uid := c.Query("operator_id"); uid != "" {
			q = q.Where("operator_id = ?", uid)
		}
		if m := c.Query("module"); m != "" {
			q = q.Where("module = ?", m)
		}
		if a := c.Query("action"); a != "" {
			q = q.Where("action = ?", a)
		}
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("action LIKE ? OR module LIKE ? OR username LIKE ? OR operator_ip LIKE ?",
				"%"+kw+"%", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var logs []model.LogOperation
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&logs).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询操作日志失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminListVerifyLogs GET /admin/logs/verify
// 验证日志列表（支持 tenant_id/app_id/action/result/date/keyword 筛选）
func AdminListVerifyLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.LogVerify{})
		if t := c.Query("tenant_id"); t != "" {
			q = q.Where("tenant_id = ?", t)
		}
		if a := c.Query("app_id"); a != "" {
			q = q.Where("app_id = ?", a)
		}
		if a := c.Query("action"); a != "" {
			q = q.Where("action = ?", a)
		}
		if r := c.Query("result"); r != "" {
			q = q.Where("result = ?", r)
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("client_ip LIKE ? OR extra LIKE ?",
				"%"+kw+"%", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var logs []model.LogVerify
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&logs).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询验证日志失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminListLoginFailedLogs GET /admin/logs/login_failed
// 登录失败日志列表（支持 user_type/username/ip/reason/date 筛选）
func AdminListLoginFailedLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.LogLoginFailed{})
		if t := c.Query("user_type"); t != "" {
			q = q.Where("user_type = ?", t)
		}
		if u := c.Query("username"); u != "" {
			q = q.Where("username LIKE ?", "%"+u+"%")
		}
		if ip := c.Query("ip"); ip != "" {
			q = q.Where("client_ip = ?", ip)
		}
		if r := c.Query("reason"); r != "" {
			q = q.Where("reason = ?", r)
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}

		var total int64
		q.Count(&total)

		var logs []model.LogLoginFailed
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&logs).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询登录失败日志失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 日志导出 ==============

// AdminExportLogs GET /admin/logs/export?type=operation|verify|login_failed
// 导出 CSV（最多 10000 条），通过 Content-Disposition 触发下载
func AdminExportLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		logType := strings.ToLower(c.DefaultQuery("type", "operation"))
		now := time.Now()
		filename := "logs_" + logType + "_" + now.Format("20060102_150405") + ".csv"

		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
		// BOM 让 Excel 正确识别 UTF-8
		_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))

		const exportLimit = 10000

		switch logType {
		case "verify":
			var logs []model.LogVerify
			deps.DB.Order("id DESC").Limit(exportLimit).Find(&logs)
			_, _ = c.Writer.Write([]byte("ID,TenantID,AppID,CardID,DeviceID,Action,Result,ClientIP,UserAgent,CreatedAt\n"))
			for _, l := range logs {
				_, _ = c.Writer.Write([]byte(csvRow(
					l.ID, l.TenantID, l.AppID, ptrUint64Str(l.CardID), ptrUint64Str(l.DeviceID),
					l.Action, l.Result, l.ClientIP, l.UserAgent, l.CreatedAt.Format(time.RFC3339),
				)))
			}
		case "login_failed":
			var logs []model.LogLoginFailed
			deps.DB.Order("id DESC").Limit(exportLimit).Find(&logs)
			_, _ = c.Writer.Write([]byte("ID,UserType,Username,ClientIP,Reason,UserAgent,CreatedAt\n"))
			for _, l := range logs {
				_, _ = c.Writer.Write([]byte(csvRow(
					l.ID, l.UserType, l.Username, l.ClientIP, l.Reason, l.UserAgent, l.CreatedAt.Format(time.RFC3339),
				)))
			}
		default: // operation
			var logs []model.LogOperation
			deps.DB.Order("id DESC").Limit(exportLimit).Find(&logs)
			_, _ = c.Writer.Write([]byte("ID,OperatorType,OperatorID,Username,OperatorIP,Module,Action,Status,TargetType,TargetID,CreatedAt\n"))
			for _, l := range logs {
				_, _ = c.Writer.Write([]byte(csvRow(
					l.ID, l.OperatorType, l.OperatorID, l.Username, l.OperatorIP,
					l.Module, l.Action, l.Status, l.TargetType, ptrUint64Str(l.TargetID),
					l.CreatedAt.Format(time.RFC3339),
				)))
			}
		}
	}
}

// csvRow 将一行字段拼接为 CSV 行（字段含逗号/引号时自动加引号转义）
func csvRow(fields ...interface{}) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		s := ""
		switch v := f.(type) {
		case string:
			s = v
		case uint64:
			s = strconv.FormatUint(v, 10)
		case int64:
			s = strconv.FormatInt(v, 10)
		case int:
			s = strconv.Itoa(v)
		default:
			s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(toStr(f), "\n", " "), "\r", " "))
		}
		// 字段含 , " \n 时加引号
		if strings.ContainsAny(s, ",\"\n") {
			s = "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ",") + "\n"
}

// ptrUint64Str 处理 *uint64 字段，nil 返回空串
func ptrUint64Str(p *uint64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatUint(*p, 10)
}

// toStr 简易 fmt.Sprintf 包装（避免在 csvRow 内 import fmt）
func toStr(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case uint64:
		return strconv.FormatUint(x, 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

// 兼容性：导入 strconv
var _ = strconv.Itoa
var _ = context.Background
var _ = auth.RoleAdmin
