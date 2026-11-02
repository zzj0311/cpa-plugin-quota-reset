package activator

import (
	"encoding/json"
	"fmt"
	"strings"

	"quota-activation/internal/host"
)

// EvaluateAntigravityActivationSuccess 严格判定 Antigravity 唤醒是否成功。
// 条件：HTTP 2xx + 顶层或 response.candidates 任一非空 + 无 error 对象。
func EvaluateAntigravityActivationSuccess(statusCode int, body []byte) (ok bool, message string) {
	if !host.IsHTTPSuccess(statusCode) {
		return false, fmt.Sprintf("Antigravity唤醒失败：上游返回非成功状态（HTTP %d）", statusCode)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return false, "Antigravity唤醒失败：响应体为空"
	}

	var payload antigravityActivationResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, "Antigravity唤醒失败：响应不是合法数据"
	}
	if payload.Error != nil {
		return false, "Antigravity唤醒失败：上游返回业务错误"
	}
	if len(payload.Candidates) == 0 && len(payload.Response.Candidates) == 0 {
		return false, "Antigravity唤醒失败：响应缺少候选项"
	}
	return true, ""
}
