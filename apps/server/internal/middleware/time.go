// 时间工具（拆分便于测试 mock）
package middleware

import "time"

// timeNow 当前时间（可被测试覆盖）
var timeNow = func() time.Time {
	return time.Now()
}
