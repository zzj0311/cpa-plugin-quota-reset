<div align="center">

# Quota-Activation

[中文](./README.md) | [English](./README.en.md)

</div>

> 此仓库是 cpaDash 的 `quota-reset` 分支。以下是原上游文档，插件 ID、安装地址和发布流程不适用于此分支；请优先阅读 [LOCAL.md](LOCAL.md)。

CLIProxyAPI (CPA) 配额唤醒插件。上游插件 ID、动态库基础名与 CPA 配置键均为 `quota-activation`。

## 导航

- [功能概览](#功能概览)
- [工作流程](#工作流程)
- [构建与安装](#构建与安装)
- [插件商店来源](#插件商店来源)
- [配置说明](#配置说明)
- [管理页面与接口](#管理页面与接口)
- [许可证](#许可证)

## 功能概览

- 支持 **Codex** 与 **Antigravity**（含 `gemini`、`claude_gpt` 与 `gemini_4_argon` 独立模型组）凭证的手动唤醒与自动扫描唤醒。
- 自动唤醒与手动唤醒配置分离：自动路径仅使用 `auto_activate`、`scan_interval`、`activation_models.*`。
- 自动额度窗口优先使用真实长窗口额度数据。
- Codex 自动解析凭据推断套餐类型，忽略 5 小时滚动短窗；缺少真实长窗口时，按套餐合成长窗口（付费 7 天，免费 / 未知 30 天）。
- Antigravity 仍使用 7 天默认长窗口，支持多组位置配对配置与凭证可用模型动态补全（基于能力提示扩展）。
- 管理页面支持中英双语、执行记录查看、手动唤醒成功/失败/跳过三态反馈与操作防重保护，全链路脱敏保障凭据安全。

## 工作流程

```text
加载插件
  -> 读取 plugins.configs.quota-activation 配置
  -> 注册 management 路由与静态资源页 /status
  -> 手动：用户选择凭证与模型后 POST /activate
  -> 自动：auto_activate=true 时按 scan_interval 扫描
       - host.auth.list / get_runtime 取凭证
       - 真实长窗口额度数据优先
       - Codex 忽略 5 小时滚动短窗；无真实长窗时按套餐合成（付费 7 天，免费 / 未知 30 天）
       - Antigravity 保留 7 天默认长窗
       - 同 cycle 已 success 则跳过（不以 5 小时短窗做硬跳过）
       - 与手动共用同一套唤醒内核
       - 默认 direct_http（host.http.do）；可选 scheduler_boost
```

## 构建与安装

插件以 CGO 动态库形式运行，宿主会从动态库文件名去掉扩展名得到插件 ID，因此文件名必须保持为 `quota-activation.<ext>`。

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o quota-activation.so .
```

或：

```bash
./scripts/build.sh
```

把产物放入 CPA 插件发现目录之一：

- `plugins/<GOOS>/<GOARCH>/quota-activation.<ext>`
- `plugins/<GOOS>/<GOARCH>-<variant>/quota-activation.<ext>`
- `plugins/quota-activation.<ext>`

扩展名：Linux/FreeBSD 为 `.so`，macOS 为 `.dylib`，Windows 为 `.dll`。

## 插件商店来源

如需通过 CPA 插件商店安装本插件，第三方来源必须指向 `registry.json` 的原始 JSON 文本：

```yaml
plugins:
  enabled: true
  store-sources:
    - "https://raw.githubusercontent.com/Cody292/quota-activation/main/registry.json"
```

不要使用 `https://github.com/Cody292/quota-activation/blob/main/registry.json`。该地址返回 GitHub HTML 页面，CPA 无法按插件商店 registry 解析。修改 `store-sources` 后，重启 CPA 或通过管理端重新加载配置，再刷新插件商店列表。

## 配置说明

在 CPA `config.yaml` 中启用插件系统，并在 `plugins.configs.quota-activation` 下保留插件自有配置：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    quota-activation:
      enabled: true
      auto_activate: false
      enable_before_activation: false
      scan_interval: "30"
      activation_request_timeout: "60"
      max_concurrency: 1
      activation_prompt: "quota activation ping"
      # 默认 direct_http：经 host.http.do 直连上游，不写 priority / 不走调度器
      activation_transport: "direct_http"
      # direct_http 仅在传输/宿主类失败时回退 legacy scheduler_boost；业务失败不回退
      scheduler_boost_fallback: true
      activation_models:
        codex:
          models: "gpt-5.4-mini"
        antigravity:
          # 支持单组（如 "gemini" / "gemini-3-flash"）或 1~3 个模型组（用 | 分隔并按位置配对，如 "gemini|gemini_4_argon" 与 "gemini-3-flash|gemini-4-argon"；每个组只能一个模型，不支持数组/逗号）
          models_group: "gemini"
          models: "gemini-3-flash"
```

字段说明：

| 字段 | 说明 |
| :--- | :--- |
| `enabled` | 单插件开关；还需要全局 `plugins.enabled: true` 且动态库注册成功。 |
| `auto_activate` | 是否启用自动配额唤醒；默认 `false`，手动唤醒仍是默认方式。 |
| `enable_before_activation` | 为 `true` 时，达到唤醒条件后自动启用已禁用凭证并保持启用。 |
| `scan_interval` | 自动扫描间隔，**单位：分钟**。填写纯数字即可，无需带单位；默认 `30`。 |
| `activation_request_timeout` | 唤醒请求超时，**单位：秒**。填写纯数字即可；默认 `60`。 |
| `max_concurrency` | 自动扫描与激活的并发上限（worker 池大小）；默认 `1`。 |
| `activation_prompt` | 唤醒提示词（`direct_http` 写入上游请求体；`scheduler_boost` 经 `host.model.execute` 发送）。 |
| `activation_transport` | 唤醒传输方式：`direct_http`（**默认**，`host.http.do` 直连）或 `scheduler_boost`（legacy：临时提升 priority + `host.model.execute`）。 |
| `scheduler_boost_fallback` | 仅当 `activation_transport=direct_http` 时生效；默认 `true`。遇**传输/宿主类**失败可回退一次 `scheduler_boost`；**业务失败 / 假 2xx（无有效结构）禁止回退**，也不得写入 success cycle。 |
| `activation_models.codex.models` | Codex 自动唤醒模型名称。 |
| `activation_models.antigravity.models_group` | Antigravity 自动唤醒模型组（`gemini`、`claude_gpt`、`gemini_4_argon`）。支持 1~3 个模型组并用 `\|` 分隔，与 `models` 按位置配对；每个组只能一个模型；旧单值仍兼容；不支持数组/逗号。 |
| `activation_models.antigravity.models` | Antigravity 自动唤醒模型名称。多组时同样用 `\|` 分隔并与 `models_group` 按位置配对；每个组只能一个模型；旧单值仍兼容；不支持数组/逗号。 |

说明：

- 仍可写带单位的 Go duration（如 `45m`、`2h`、`90s`）。
- 自动扫描与手动唤醒共用 `Activator.Activate` 内核；`scan_interval` 门闩允许约 2 秒微早触发，避免 timer 抖动静默丢轮。
- 管理页、诊断与执行历史的 `last_error` / 失败原因均为**纯中文**（旧英文串会映射）。

## 管理页面与接口

### 资源页面

- `GET /v0/resource/plugins/quota-activation/status`
  返回静态 HTML 页面，用于管理密钥验证、凭证选择与手动唤醒。页面内动态请求走 management API。

### 管理 API

以下接口需要 CPA 管理密钥：

- `GET /v0/management/quota-activation/status`
- `GET /v0/management/quota-activation/auth-files`
- `POST /v0/management/quota-activation/activate`
- `GET /v0/management/quota-activation/diagnostics`

模型列表使用宿主全局接口：

- `GET /v0/management/auth-files/models?name=...`

## 许可证

本项目使用 MIT License，详见 [LICENSE](./LICENSE)。
