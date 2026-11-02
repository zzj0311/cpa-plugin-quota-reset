package activator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// AuthMaterial 是从物理凭证 JSON 短时解析出的上游调用材料。
// 禁止写入日志或错误串；String/Error 路径不得回显令牌。
type AuthMaterial struct {
	AccessToken  string
	RefreshToken string
	AccountID    string
	ProjectID    string
}

// ErrAuthMaterial 表示凭证材料解析失败（用户可见纯中文）。
var ErrAuthMaterial = errors.New("凭证材料无效")

// ParseAuthMaterial 从 host.auth.get 物理 JSON 解析令牌与账号字段。
// 支持顶层与 tokens 嵌套；禁止在错误中回显令牌原文。
func ParseAuthMaterial(raw []byte) (AuthMaterial, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return AuthMaterial{}, fmt.Errorf("%w：凭证内容为空", ErrAuthMaterial)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return AuthMaterial{}, fmt.Errorf("%w：凭证不是合法数据", ErrAuthMaterial)
	}

	material := AuthMaterial{
		AccessToken: firstStringField(root,
			"access_token", "accessToken", "oauth_access_token", "oauthAccessToken",
			"api_key", "apiKey", "session_token", "sessionToken", "token", "id_token", "idToken",
		),
		RefreshToken: firstStringField(root,
			"refresh_token", "refreshToken", "oauth_refresh_token", "oauthRefreshToken",
		),
		AccountID: firstStringField(root, "account_id", "chatgpt_account_id", "accountId", "chatgptAccountId"),
		ProjectID: firstStringField(root, "project_id", "projectId", "project", "quota_project_id", "quotaProjectID"),
	}

	if tokensRaw, ok := root["tokens"]; ok {
		var tokens map[string]json.RawMessage
		if json.Unmarshal(tokensRaw, &tokens) == nil {
			if material.AccessToken == "" {
				material.AccessToken = firstStringField(tokens,
					"access_token", "accessToken", "oauth_access_token", "oauthAccessToken",
					"api_key", "apiKey", "session_token", "sessionToken", "token", "id_token", "idToken",
				)
			}
			if material.RefreshToken == "" {
				material.RefreshToken = firstStringField(tokens,
					"refresh_token", "refreshToken", "oauth_refresh_token", "oauthRefreshToken",
				)
			}
		}
	}

	// credentials / auth 嵌套对象（部分宿主导出形态）
	for _, nestKey := range []string{"credentials", "auth", "oauth", "session"} {
		if material.AccessToken != "" {
			break
		}
		nestedRaw, ok := root[nestKey]
		if !ok {
			continue
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(nestedRaw, &nested) != nil {
			continue
		}
		if material.AccessToken == "" {
			material.AccessToken = firstStringField(nested,
				"access_token", "accessToken", "api_key", "apiKey", "session_token", "sessionToken", "token",
			)
		}
		if material.RefreshToken == "" {
			material.RefreshToken = firstStringField(nested, "refresh_token", "refreshToken")
		}
		if material.AccountID == "" {
			material.AccountID = firstStringField(nested, "account_id", "accountId", "chatgpt_account_id")
		}
		if material.ProjectID == "" {
			material.ProjectID = firstStringField(nested, "project_id", "projectId", "project", "quota_project_id", "quotaProjectID")
		}
	}

	if strings.TrimSpace(material.AccessToken) == "" {
		return AuthMaterial{}, fmt.Errorf("%w：缺少访问令牌", ErrAuthMaterial)
	}
	return material, nil
}

// String 返回不含令牌的调试摘要（禁止日志打 token）。
func (m AuthMaterial) String() string {
	return fmt.Sprintf("AuthMaterial{has_access=%t has_refresh=%t account=%q project=%q}",
		strings.TrimSpace(m.AccessToken) != "",
		strings.TrimSpace(m.RefreshToken) != "",
		m.AccountID,
		m.ProjectID,
	)
}

func firstStringField(object map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := object[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
