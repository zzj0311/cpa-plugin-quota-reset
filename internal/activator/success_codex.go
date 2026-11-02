package activator

import (
	"encoding/json"
	"fmt"
	"strings"

	"quota-activation/internal/host"
)

// EvaluateCodexActivationSuccess 严格判定 Codex 唤醒是否成功。
// 订阅 OAuth 端点返回 SSE（stream=true）；亦兼容单 JSON 响应。
// 不得仅依赖 HTTP 2xx：须存在 response id / completed / output 结构且无 error。
func EvaluateCodexActivationSuccess(statusCode int, body []byte) (ok bool, message string) {
	if !host.IsHTTPSuccess(statusCode) {
		return false, fmt.Sprintf("Codex唤醒失败：上游返回非成功状态（HTTP %d）", statusCode)
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false, "Codex唤醒失败：响应体为空"
	}

	// SSE：event/data 行；优先 response.completed / response.failed。
	if strings.Contains(trimmed, "event:") || strings.Contains(trimmed, "data:") {
		return evaluateCodexSSESuccess(trimmed)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return false, "Codex唤醒失败：响应不是合法数据"
	}

	if errMsg, _, hasError := codexErrorFromRoot(root); hasError {
		return false, errMsg
	}

	if codexHasStructuralSuccess(root) {
		return true, ""
	}
	if wrapped, ok := root["response"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(wrapped, &nested) == nil {
			if errMsg, _, hasError := codexErrorFromRoot(nested); hasError {
				return false, errMsg
			}
			if codexHasStructuralSuccess(nested) {
				return true, ""
			}
		}
	}
	return false, "Codex唤醒失败：响应缺少有效输出结构"
}

func evaluateCodexSSESuccess(sse string) (bool, string) {
	var lastResponse map[string]json.RawMessage
	sawCompleted := false
	for _, line := range strings.Split(sse, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		eventType := ""
		if raw, ok := event["type"]; ok {
			_ = json.Unmarshal(raw, &eventType)
		}
		if raw, ok := event["response"]; ok {
			var nested map[string]json.RawMessage
			if json.Unmarshal(raw, &nested) == nil {
				lastResponse = nested
			}
		}
		switch eventType {
		case "response.failed", "error":
			if lastResponse != nil {
				if errMsg, _, hasError := codexErrorFromRoot(lastResponse); hasError {
					return false, errMsg
				}
			}
			if errMsg, _, hasError := codexErrorFromRoot(event); hasError {
				return false, errMsg
			}
			return false, "Codex唤醒失败：上游返回业务错误"
		case "response.completed":
			sawCompleted = true
		}
	}
	if lastResponse != nil {
		if errMsg, _, hasError := codexErrorFromRoot(lastResponse); hasError {
			return false, errMsg
		}
		if codexHasStructuralSuccess(lastResponse) || sawCompleted {
			return true, ""
		}
		// completed 事件中 status=completed 且带 id 也算成功。
		if statusRaw, ok := lastResponse["status"]; ok {
			var status string
			if json.Unmarshal(statusRaw, &status) == nil && status == "completed" {
				if idRaw, ok := lastResponse["id"]; ok {
					var id string
					if json.Unmarshal(idRaw, &id) == nil && strings.TrimSpace(id) != "" {
						return true, ""
					}
				}
			}
		}
	}
	if sawCompleted {
		return true, ""
	}
	// 流中出现 response.created 且带 id 视为结构成功（部分宿主截断末尾事件）。
	if strings.Contains(sse, `"type":"response.created"`) && strings.Contains(sse, `"id":"resp_`) {
		return true, ""
	}
	return false, "Codex唤醒失败：响应缺少有效输出结构"
}

func codexHasStructuralSuccess(root map[string]json.RawMessage) bool {
	if idRaw, ok := root["id"]; ok {
		var id string
		if json.Unmarshal(idRaw, &id) == nil && strings.TrimSpace(id) != "" {
			return true
		}
	}
	if outputRaw, ok := root["output"]; ok {
		var output []json.RawMessage
		if json.Unmarshal(outputRaw, &output) == nil && len(output) > 0 {
			return true
		}
	}
	return false
}

func codexErrorFromRoot(root map[string]json.RawMessage) (message string, usageLimit bool, hasError bool) {
	errRaw, ok := root["error"]
	if !ok || string(errRaw) == "null" {
		return "", false, false
	}
	var payload struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	// error 可能是对象或字符串。
	if err := json.Unmarshal(errRaw, &payload); err != nil {
		var asString string
		if json.Unmarshal(errRaw, &asString) == nil && strings.TrimSpace(asString) != "" {
			return "Codex唤醒失败：上游返回错误", false, true
		}
		return "Codex唤醒失败：上游返回错误", false, true
	}
	typeText := strings.ToLower(strings.TrimSpace(payload.Type))
	codeText := strings.ToLower(strings.TrimSpace(payload.Code))
	msgText := strings.ToLower(strings.TrimSpace(payload.Message))
	if typeText == usageLimitReachedType || codeText == usageLimitReachedType ||
		strings.Contains(typeText, "usage_limit") || strings.Contains(codeText, "usage_limit") ||
		strings.Contains(msgText, "usage limit") || strings.Contains(msgText, "usage_limit") {
		return "Codex唤醒失败：用量额度已耗尽", true, true
	}
	return "Codex唤醒失败：上游返回业务错误", false, true
}
