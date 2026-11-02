package activator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// CodexActivationURL 是 ChatGPT Codex 订阅 OAuth 唤醒端点。
// 与 Antigravity / quota 探测 URL 完全独立，禁止混用。
const CodexActivationURL = "https://chatgpt.com/backend-api/codex/responses"

// ErrCodexProtocol 表示 Codex 协议构建失败（品牌名保留英文，其余中文）。
var ErrCodexProtocol = errors.New("Codex协议无效")

// codexActivationBody 是 Codex 订阅 OAuth 唤醒报文。
// 上游强制 stream=true、store=false；input 须为 message 列表（非 chat content 字符串）。
type codexActivationBody struct {
	Model        string              `json:"model"`
	Instructions string              `json:"instructions"`
	Input        []codexInputMessage `json:"input"`
	Store        bool                `json:"store"`
	Stream       bool                `json:"stream"`
}

type codexInputMessage struct {
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []codexInputPart   `json:"content"`
}

type codexInputPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// BuildCodexProtocol 构建 Codex 直连唤醒请求。
// 规则：POST codex/responses + store=false + stream=true + message/input_text；禁止 messages/project。
func BuildCodexProtocol(material AuthMaterial, model, prompt string) (ProtocolRequest, error) {
	token := strings.TrimSpace(material.AccessToken)
	if token == "" {
		return ProtocolRequest{}, fmt.Errorf("%w：缺少访问令牌", ErrCodexProtocol)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return ProtocolRequest{}, fmt.Errorf("%w：缺少模型名称", ErrCodexProtocol)
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "hi"
	}

	body, err := json.Marshal(codexActivationBody{
		Model:        model,
		Instructions: "You are a helpful assistant.",
		Input: []codexInputMessage{{
			Type: "message",
			Role: "user",
			Content: []codexInputPart{{
				Type: "input_text",
				Text: prompt,
			}},
		}},
		Store:  false,
		Stream: true,
	})
	if err != nil {
		return ProtocolRequest{}, fmt.Errorf("%w：编码请求失败", ErrCodexProtocol)
	}

	headers := map[string][]string{
		"Accept":        {"text/event-stream"},
		"Authorization": {"Bearer " + token},
		"Content-Type":  {"application/json"},
		"OpenAI-Beta":   {"responses=v1"},
		"originator":    {"codex_cli_rs"},
		"User-Agent":    {"codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal"},
	}
	if accountID := strings.TrimSpace(material.AccountID); accountID != "" {
		headers["Chatgpt-Account-Id"] = []string{accountID}
	}

	return ProtocolRequest{
		Method:  http.MethodPost,
		URL:     CodexActivationURL,
		Headers: headers,
		Body:    body,
	}, nil
}
