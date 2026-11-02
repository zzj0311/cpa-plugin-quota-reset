package activator

import (
	"strings"
)

// LocalizeUserMessage 将内部/英文错误串映射为用户可见纯中文。
// 已是中文则尽量原样返回；未知英文兜底为通用失败文案，避免界面中英混排。
func LocalizeUserMessage(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return msg
	}
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "usage_limit") || strings.Contains(lower, "usage limit"):
		return "Codex唤醒失败：用量额度已耗尽"
	case strings.Contains(lower, "activator: busy") || strings.EqualFold(msg, ErrBusy.Error()):
		return "执行中"
	case strings.Contains(lower, "activator: invalid request") || strings.Contains(lower, "missing auth_id") ||
		strings.Contains(lower, "missing cycle_key") || strings.Contains(lower, "missing provider") ||
		strings.Contains(lower, "missing window") || strings.HasPrefix(lower, "missing "):
		return "唤醒请求无效：缺少必要字段"
	case strings.Contains(lower, "activator: missing dependency"):
		return "唤醒失败：缺少内部依赖"
	case strings.Contains(lower, "activator: auth file not found"):
		return "未找到匹配的凭证文件"
	case strings.Contains(lower, "activator: disabled credential"):
		return "凭证已禁用"
	case strings.Contains(lower, "create activation session"):
		return "创建唤醒会话失败"
	case strings.Contains(lower, "encode activation ping"):
		return "编码唤醒请求失败"
	case strings.Contains(lower, "available model is required"):
		return "缺少可用模型"
	case strings.Contains(lower, "list auth files for available models"):
		return "列举凭证以解析可用模型失败"
	case strings.Contains(lower, "list auth files:") || strings.Contains(lower, "list auth files "):
		return "列举凭证失败"
	case strings.Contains(lower, "enable auth file"):
		return "启用凭证失败"
	case strings.Contains(lower, "decode auth file"):
		return "解析凭证文件失败"
	case strings.Contains(lower, "encode disabled flag") || strings.Contains(lower, "encode auth file"):
		return "写入凭证文件失败"
	case strings.Contains(lower, "host model executor is unavailable") ||
		(strings.Contains(lower, "host model execute") && strings.Contains(lower, "unavailable")) ||
		(strings.Contains(lower, "host_call_failed") && strings.Contains(lower, "executor")):
		return "宿主模型执行器不可用（宿主未就绪或回调失败，非凭证失效）"
	case strings.Contains(lower, "host model execute failed"):
		return "宿主模型执行失败"
	case strings.Contains(lower, "host model execute"):
		return "宿主模型执行失败"
	case strings.Contains(lower, "host callback host.model.execute"):
		return "宿主模型执行失败"
	case strings.Contains(lower, "activation error"):
		return "唤醒失败"
	case strings.Contains(lower, "antigravity activation response missing choices"):
		return "Antigravity唤醒失败：响应缺少选项"
	case strings.Contains(lower, "save activation state") || strings.Contains(lower, "save state context"):
		return "唤醒已成功，但状态保存被中断（内部超时/取消），非凭证失效"
	case strings.Contains(lower, "context canceled") || strings.Contains(lower, "context cancelled"):
		return "操作已取消"
	case strings.Contains(lower, "context deadline exceeded"):
		return "操作超时"
	case strings.Contains(lower, "priority boost required but auth document incomplete"):
		return "需要提升优先级，但未能读取完整凭证文件"
	case strings.Contains(lower, "priority boost refused incomplete auth document"):
		return "凭证文档不完整，拒绝写入优先级"
	case strings.Contains(lower, "is not unique highest priority") ||
		(strings.Contains(lower, "runtime priority") && strings.Contains(lower, "expected")) ||
		strings.Contains(lower, "priority boost confirm failed") ||
		strings.Contains(lower, "runtime auth not found"):
		return "优先级提升后，运行时仍未确认目标为最高优先级"
	case strings.Contains(lower, "priority boost confirm canceled"):
		return "优先级确认已取消"
	case strings.Contains(lower, "priority boost confirm: missing host"):
		return "优先级确认失败：宿主不可用"
	case strings.Contains(lower, "get physical auth file") ||
		strings.Contains(lower, "list auth files for priority boost"):
		return "读取物理凭证文件失败"
	case strings.Contains(lower, "boost auth priority"):
		return "提升凭证优先级失败"
	case strings.Contains(lower, "auth document is empty") ||
		strings.Contains(lower, "auth document has no fields"):
		return "凭证文档不完整，拒绝写入优先级"
	case strings.Contains(lower, "connection reset by peer") ||
		strings.Contains(lower, "activator: network failure"):
		return "网络连接失败"
	}

	// 品牌名 Codex/Antigravity + 中文说明：已是用户可见终态，原样返回。
	if isBrandPrefixedUserMessage(msg) {
		return msg
	}
	// 已是纯中文（无拉丁字母）则直接返回。
	if !containsLatinLetter(msg) {
		return msg
	}
	// 中文前缀 + 英文包装：尽量保留中文前缀段。
	if idx := strings.Index(msg, "："); idx >= 0 {
		head := strings.TrimSpace(msg[:idx])
		if head != "" && !containsLatinLetter(head) {
			return head
		}
		tail := strings.TrimSpace(msg[idx+len("："):])
		if tail != "" && !containsLatinLetter(tail) {
			return tail
		}
	}
	return "唤醒失败"
}

// isBrandPrefixedUserMessage 判定是否已是「品牌英文 + 中文说明」的终态文案。
func isBrandPrefixedUserMessage(msg string) bool {
	for _, prefix := range []string{"Codex", "Antigravity"} {
		if !strings.HasPrefix(msg, prefix) {
			continue
		}
		rest := strings.TrimPrefix(msg, prefix)
		if rest == "" {
			return false
		}
		// 去掉品牌后不得再含其它拉丁字母（允许数字/中文/标点，及 HTTP 状态码中的 HTTP）。
		stripped := strings.ReplaceAll(rest, "HTTP", "")
		for _, r := range stripped {
			if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				return false
			}
		}
		return true
	}
	return false
}
