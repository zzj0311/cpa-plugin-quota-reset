package planclaim

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// Type 表示从物理凭证 JWT 解析出的 ChatGPT 套餐类别。
type Type string

const (
	// TypeFree 表示免费套餐。
	TypeFree Type = "free"
	// TypePaid 表示付费套餐（plus / pro / team）。
	TypePaid Type = "paid"
	// TypeUnknown 表示套餐未知、缺失或凭证畸形。
	TypeUnknown Type = "unknown"
)

const (
	openaiAuthClaim  = "https://api.openai.com/auth"
	windowMonthly    = "monthly"
	windowWeekly     = "weekly"
	monthlyLimitSecs = 30 * 24 * 60 * 60
	weeklyLimitSecs  = 7 * 24 * 60 * 60
	idTokenKey       = "id_token"
	idTokenCamelKey  = "idToken"
	accessTokenKey   = "access_token"
	claimPlanType    = "chatgpt_plan_type"
	claimPlanTypeAlt = "plan_type"
)

// String 返回套餐类别的稳定标识，不含任何令牌原文。
func (plan Type) String() string {
	return string(plan)
}

// FromAuthJSON 从物理凭证 JSON 提取 chatgpt_plan_type 并映射为套餐类别。
// 只解码 JWT payload，不验签、不联网；无法识别时返回 TypeUnknown。
func FromAuthJSON(raw []byte) Type {
	root, ok := parseRoot(raw)
	if !ok {
		return TypeUnknown
	}
	// 优先读 id_token/idToken；仅当缺失时才回退 3 段 JWT 形态的 access_token。
	if token, found := lookupString(root, idTokenKey, idTokenCamelKey); found {
		return classifyJWT(token)
	}
	if token, found := lookupString(root, accessTokenKey); found && isThreeSegmentJWT(token) {
		return classifyJWT(token)
	}
	return TypeUnknown
}

// WindowFor 返回套餐对应的窗策略名称与时长（秒）。
// free 与 unknown 为 monthly 2592000；paid 为 weekly 604800。
func WindowFor(plan Type) (name string, limitSeconds int) {
	if plan == TypePaid {
		return windowWeekly, weeklyLimitSecs
	}
	return windowMonthly, monthlyLimitSecs
}

func classifyJWT(token string) Type {
	plan, ok := planTypeFromJWT(token)
	if !ok {
		return TypeUnknown
	}
	return mapPlan(plan)
}

func mapPlan(raw string) Type {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "free":
		return TypeFree
	case "plus", "pro", "team":
		return TypePaid
	default:
		return TypeUnknown
	}
}

func planTypeFromJWT(token string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return "", false
	}
	payload, err := decodeJWTPayload(parts[1])
	if err != nil {
		return "", false
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", false
	}
	if auth, ok := asObject(claims[openaiAuthClaim]); ok {
		if plan, ok := stringField(auth, claimPlanType); ok {
			return plan, true
		}
	}
	if plan, ok := stringField(claims, claimPlanType); ok {
		return plan, true
	}
	return stringField(claims, claimPlanTypeAlt)
}

// decodeJWTPayload 仅解码 payload 段：RawURLEncoding，并去掉 padding。
func decodeJWTPayload(segment string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(segment, "="))
}

func parseRoot(raw []byte) (map[string]any, bool) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, false
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return nil, false
	}
	return root, true
}

func lookupString(root map[string]any, keys ...string) (string, bool) {
	if value, ok := firstString(root, keys...); ok {
		return value, true
	}
	for _, nest := range []string{"tokens", "credentials", "auth", "oauth"} {
		child, ok := asObject(root[nest])
		if !ok {
			continue
		}
		if value, ok := firstString(child, keys...); ok {
			return value, true
		}
	}
	return "", false
}

func firstString(object map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := stringField(object, key); ok {
			return value, true
		}
	}
	return "", false
}

func stringField(object map[string]any, key string) (string, bool) {
	raw, ok := object[key]
	if !ok {
		return "", false
	}
	text, ok := raw.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}

func asObject(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

func isThreeSegmentJWT(token string) bool {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return false
	}
	return parts[0] != "" && parts[1] != "" && parts[2] != ""
}
