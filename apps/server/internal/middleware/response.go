// 统一响应格式 + 错误码（详见 SPEC.md 3.5 节）
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Response 统一响应结构
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp int64       `json:"timestamp"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: getRequestID(c),
		Timestamp: now(),
	})
}

// Fail 失败响应
func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:      code,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: now(),
	})
}

// getRequestID 从上下文获取请求 ID，没有则生成
func getRequestID(c *gin.Context) string {
	if id, exists := c.Get("request_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	id := "req-" + uuid.New().String()
	c.Set("request_id", id)
	return id
}

// now 当前 Unix 时间戳（拆分以便测试 mock）
func now() int64 {
	// 注：使用全局变量或注入时间函数便于测试，此处简化
	return timeNow().Unix()
}
