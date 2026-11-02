package activator

import (
	"strings"
)

// isDirectTransportOrHostFailure 判定 direct_http 失败是否属于传输/宿主类，
// 仅此类错误在 SchedulerBoostFallback=true 时可回退 legacy boost。
// 严格成功判定失败、usage_limit 等业务结果不得伪装成功，禁止 fallback。
func isDirectTransportOrHostFailure(result Result, activateErr error) bool {
	if activateErr != nil {
		// wakeDirect 当前对业务失败返回 (result, nil)；非 nil 错误视为宿主/依赖异常。
		return true
	}
	// 401 鉴权跳过 / 成功：禁止 fallback。
	if result.Status == StatusSkipped || result.Status == StatusBusy || result.Success {
		return false
	}
	if result.Status != StatusFailed {
		return false
	}
	msg := strings.TrimSpace(result.LastError)
	if msg == "" {
		return false
	}
	// 业务 / 严格成功判定 / 鉴权失效：禁止 fallback。
	if isDirectBusinessFailureMessage(msg) {
		return false
	}
	// 传输 / 宿主类：允许 fallback。
	return isDirectTransportOrHostMessage(msg)
}

func isDirectBusinessFailureMessage(msg string) bool {
	lower := strings.ToLower(msg)
	businessMarkers := []string{
		"用量额度",
		"usage_limit",
		"usage limit",
		"上游判定未通过",
		"响应缺少有效输出结构",
		"响应体为空",
		"响应不是合法数据",
		"上游返回业务错误",
		"上游返回错误",
		"上游返回非成功状态",
		"凭证已失效",
		"鉴权失败",
		"凭证材料无效",
		"协议构建失败",
		"不支持的提供方",
		"缺少模型名称",
		"缺少凭证标识",
		"凭证内容为空",
	}
	for _, marker := range businessMarkers {
		if strings.Contains(msg, marker) || strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func isDirectTransportOrHostMessage(msg string) bool {
	markers := []string{
		"上游请求失败",
		"无法读取指定凭证",
		"缺少宿主依赖",
	}
	for _, marker := range markers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// shouldFallbackToSchedulerBoost：direct_http 主路径失败且开关开启且为传输/宿主类错误。
func (a *Activator) shouldFallbackToSchedulerBoost(result Result, activateErr error) bool {
	if a == nil || !a.config.SchedulerBoostFallback {
		return false
	}
	// 已在 legacy 主路径时不再二次 fallback。
	if a.usesSchedulerBoostTransport() {
		return false
	}
	return isDirectTransportOrHostFailure(result, activateErr)
}
