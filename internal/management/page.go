package management

import (
	"net/http"
)

// allow: SIZE_OK - 内嵌静态资源页以单 HTML 载荷交给 CPA resources 路由；拆分不会降低运行时职责。
func (h *Handler) handlePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>配额唤醒</title>
  <style>
    :root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f8fafc; color: #111827; }
    main { max-width: 1200px; margin: 0 auto; padding: 48px 24px; }
    section { position: relative; background: #fff; border: 1px solid #e5e7eb; border-radius: 16px; padding: 24px; box-shadow: 0 12px 30px rgba(15, 23, 42, .08); }
    h1 { margin: 0; font-size: 24px; white-space: nowrap; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
    .version-badge { display: inline-flex; align-items: center; min-height: 24px; border-radius: 999px; padding: 3px 9px; background: #eff6ff; color: #1d4ed8; font-size: 12px; font-weight: 750; letter-spacing: 0; }
    p { line-height: 1.7; color: #4b5563; }
    code { background: #f3f4f6; border-radius: 6px; padding: 2px 6px; }
    label { display: block; margin: 18px 0 8px; color: #374151; font-weight: 600; }
    input, select { width: 100%; box-sizing: border-box; border: 1px solid #d1d5db; border-radius: 10px; padding: 12px; font: inherit; }
    .header-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 24px; }
    .search-container { flex: 1; display: flex; justify-content: center; }
    .search-container input { width: 100%; max-width: 280px; padding: 10px 14px; border-radius: 10px; border: 1px solid #d1d5db; font-size: 14px; }
    .top-actions { display: flex; align-items: center; gap: 12px; }
    .language-shell { position: relative; display: inline-flex; align-items: center; }
    .language-menu-button { min-height: 40px; display: inline-flex; align-items: center; gap: 8px; border: 1px solid #d1d5db; border-radius: 999px; padding: 0 14px; background: #fff; color: #374151; font-size: 14px; font-weight: 700; box-shadow: 0 1px 2px rgba(15, 23, 42, .06); }
    .language-menu-button svg { width: 16px; height: 16px; }
    .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 12px; margin-top: 16px; }
    .message { min-height: 24px; margin-top: 14px; color: #374151; white-space: pre-wrap; }
    .toast { position: absolute; top: 24px; right: 24px; z-index: 1000; max-width: min(360px, calc(100vw - 48px)); margin: 0; padding: 12px 16px; border-radius: 12px; background: #111827; color: #fff; box-shadow: 0 18px 40px rgba(15, 23, 42, .22); pointer-events: none; }
    .toast:empty { display: none; }
    button { min-height: 44px; border: 0; border-radius: 10px; padding: 0 18px; background: #111827; color: #fff; font-weight: 600; cursor: pointer; }
    button.secondary { background: #e5e7eb; color: #111827; }
    button:disabled { opacity: .6; cursor: not-allowed; }

    .select-shell { border-radius: 10px; border: 1px solid #d1d5db; background: #fff; overflow: hidden; }
    .credential-list { max-height: 220px; overflow: auto; padding: 8px; display: flex; flex-direction: column; gap: 4px; }
    .credential-item { display: flex; align-items: center; gap: 10px; margin: 0; padding: 8px 10px; border-radius: 8px; font-weight: 500; color: #374151; cursor: pointer; }
    .credential-item:hover { background: #f3f4f6; }
    .credential-item.disabled { color: #9ca3af; cursor: not-allowed; }
    .credential-item.disabled:hover { background: transparent; }
    .credential-item input { width: auto; margin: 0; }
    .credential-item.disabled input { pointer-events: none; }
    .model-menu { position: relative; }
    .model-menu-button { width: 100%; min-height: 44px; display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid #d1d5db; border-radius: 10px; padding: 0 14px; background: #fff; color: #111827; font: inherit; font-weight: 500; text-align: left; cursor: pointer; box-shadow: 0 1px 2px rgba(15, 23, 42, .04); }
    .model-menu-button:disabled { opacity: .6; cursor: not-allowed; }
    .model-menu-panel { position: absolute; left: 0; right: 0; top: calc(100% + 6px); z-index: 40; display: none; max-height: 220px; overflow: auto; border: 1px solid #d1d5db; border-radius: 10px; background: #fff; box-shadow: 0 16px 36px rgba(15, 23, 42, .14); padding: 6px; }
    .model-menu-panel.open { display: block; }
    .model-option { width: 100%; min-height: 40px; display: block; border: 0; border-radius: 8px; padding: 8px 12px; background: transparent; color: #111827; font: inherit; text-align: left; cursor: pointer; }
    .model-option:hover, .model-option.active { background: #f3f4f6; }
    .model-option.disabled { color: #9ca3af; cursor: not-allowed; opacity: .55; }
    .model-option.disabled:hover { background: transparent; }
    .provider-tabs { display: flex; flex-wrap: wrap; gap: 8px; margin: 0 0 12px; }
    .provider-tab { min-height: 36px; border: 1px solid #d1d5db; border-radius: 999px; padding: 0 14px; background: #fff; color: #374151; font-weight: 600; cursor: pointer; }
    .provider-tab.active { background: #111827; color: #fff; border-color: #111827; }

    .tabs { display: flex; gap: 8px; margin-bottom: 20px; border-bottom: 1px solid #e5e7eb; padding-bottom: 8px; }
    .tab-button { background: transparent; color: #4b5563; font-weight: 500; min-height: 38px; padding: 0 16px; border-radius: 8px; border: 1px solid transparent; cursor: pointer; }
    .tab-button:hover { background: #f3f4f6; }
    .tab-button.active { background: #111827; color: #fff; font-weight: 600; }
    .tab-panel { display: none; }
    .tab-panel.active { display: block; }
    .help-content { line-height: 1.6; color: #374151; }
    .help-content h3 { margin-top: 20px; margin-bottom: 8px; color: #111827; }
    .help-content ul { padding-left: 20px; margin-bottom: 16px; }
    .help-content li { margin-bottom: 6px; }
    .help-content pre { background: #f3f4f6; padding: 12px; border-radius: 8px; overflow-x: auto; font-family: monospace; font-size: 14px; margin: 8px 0; }

    .records-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
    .records-header-text h3 { margin: 0; font-size: 16px; color: #111827; }
    .records-hint { font-size: 13px; color: #6b7280; margin: 4px 0 0; }
    .records-list { display: grid; gap: 12px; }
    .record-card { background: #fff; border: 1px solid rgba(17,24,39,.06); border-radius: 14px; padding: 14px 16px; display: grid; gap: 10px; }
    .record-card.warn { border-color: rgba(217,119,6,.25); background: #fffbeb; }
    .record-card.fail { border-color: rgba(220,38,38,.18); background: #fef2f2; }
    .record-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; }
    .record-meta { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
    .record-time { font-size: 12px; color: #6b7280; font-weight: 600; }
    .record-badge { display: inline-flex; align-items: center; padding: 2px 10px; border-radius: 999px; font-size: 12px; font-weight: 650; }
    .record-badge.manual { background: #eff6ff; color: #1d4ed8; }
    .record-badge.auto { background: #ecfdf5; color: #047857; }
    .record-badge.scan { background: #f3e8ff; color: #6d28d9; }
    .record-stats { display: flex; flex-wrap: wrap; gap: 8px; }
    .record-stat { display: inline-flex; align-items: center; gap: 4px; border-radius: 999px; padding: 4px 10px; background: #f8fafc; border: 1px solid #e5e7eb; font-size: 12px; font-weight: 650; color: #374151; }
    .record-providers { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
    .record-provider { display: grid; gap: 4px; padding: 10px 12px; border-radius: 12px; background: #f8fafc; border: 1px solid rgba(17,24,39,.04); min-width: 0; }
    .record-provider strong { font-size: 13px; color: #111827; }
    .record-provider span { font-size: 12px; color: #6b7280; }
    .record-provider .prov-err { color: #dc2626; }
    .record-provider .prov-warn { color: #b45309; }
    .record-message { font-size: 13px; word-break: break-word; line-height: 1.5; }
    .record-message.error { color: #dc2626; }
    .record-message.warn { color: #b45309; }
    .empty-state { text-align: center; padding: 24px 12px; color: #9ca3af; font-size: 14px; }
    @media (max-width: 720px) { .record-providers { grid-template-columns: 1fr; } }
    .antigravity-groups { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 8px; }
    @media (max-width: 768px) { .antigravity-groups { grid-template-columns: 1fr; } }
    .antigravity-group-card { border: 1px solid #d1d5db; border-radius: 10px; padding: 12px; background: #fff; display: flex; flex-direction: column; gap: 6px; }
    .antigravity-group-card label { margin: 0; font-size: 13px; font-weight: 600; color: #374151; display: flex; align-items: center; justify-content: space-between; }
    .antigravity-group-card label code { font-size: 11px; }
    .antigravity-group-card select { width: 100%; border: 1px solid #d1d5db; border-radius: 8px; padding: 8px 10px; font-size: 13px; background: #fff; color: #111827; }
    .config-preview-box { margin-top: 14px; border: 1px solid #e5e7eb; border-radius: 12px; padding: 14px 16px; background: #f8fafc; }
    .preview-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; }
    .preview-title { font-size: 13px; font-weight: 700; color: #111827; }
    .preview-copy-btn { min-height: 32px; padding: 0 12px; font-size: 12px; }
    .preview-field { margin-bottom: 8px; }
    .preview-field:last-child { margin-bottom: 0; }
    .preview-field label { margin: 0 0 4px; font-size: 12px; font-weight: 500; color: #4b5563; }
    .preview-field input { background: #fff; font-family: monospace; font-size: 13px; padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; color: #111827; }
  </style>
</head>
<body>
	  <main>
	    <section>
	      <div id="loginGate">
	        <input id="managementKey" type="password" autocomplete="current-password" data-i18n-placeholder="managementKeyPlaceholder" placeholder="输入 CPA 管理密钥">
	        <div class="actions"><button type="button" data-i18n="verifyKey" onclick="verifyManagementKey(this)">验证密钥</button></div>
	      </div>
	      <div id="appShell" hidden>
	        <div class="header-bar">
				<h1><span data-i18n="title">配额唤醒</span><span class="version-badge">v0.0.12</span></h1>
	          <div class="search-container">
	            <input id="credentialSearch" type="search" data-i18n-placeholder="searchPlaceholder" placeholder="搜索凭证..." oninput="renderAuthFiles()">
	          </div>
	          <div class="top-actions">
	            <div class="language-shell">
	              <button id="languageMenuButton" type="button" class="language-menu-button" onclick="switchLanguage()" aria-label="Switch language">
	                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
	                  <circle cx="12" cy="12" r="10"></circle>
	                  <path d="M2 12h20"></path>
	                  <path d="M12 2a15.3 15.3 0 0 1 0 20"></path>
	                  <path d="M12 2a15.3 15.3 0 0 0 0 20"></path>
	                </svg>
	              </button>
	            </div>
	          </div>
	        </div>
	        <p id="managementToast" class="toast" role="status"></p>

	        <div class="tabs">
	          <button id="manualTabButton" class="tab-button active" onclick="switchTab('manual')" data-i18n="manualTab">手动唤醒</button>
	          <button id="recordsTabButton" class="tab-button" onclick="switchTab('records')" data-i18n="recordsTab">执行记录</button>
	          <button id="helpTabButton" class="tab-button" onclick="switchTab('help')" data-i18n="helpTab">帮助</button>
	        </div>

	        <div id="manualPanel" class="tab-panel active">
	          <label for="credentialSelect" data-i18n="credential">凭证</label>
	          <div id="providerTabs" class="provider-tabs"></div>
	          <div id="credentialSelect" class="select-shell credential-list" role="group" aria-label="credentials"></div>

	          <div id="singleModelSection">
	            <label for="modelMenuButton" data-i18n="model">唤醒模型</label>
	            <div class="model-menu" id="modelSelect">
	              <button id="modelMenuButton" type="button" class="model-menu-button" onclick="toggleModelMenu()" aria-haspopup="listbox" aria-expanded="false">
	                <span id="modelMenuLabel">—</span>
	                <span aria-hidden="true">▾</span>
	              </button>
	              <div id="modelMenuPanel" class="model-menu-panel" role="listbox"></div>
	            </div>
	          </div>

	          <div id="antigravityModelSection" style="display: none;">
	            <label data-i18n="antigravityModelGroups">Antigravity 模型组配置（每组最多选一个，可选 0..3 组）</label>
	            <div class="antigravity-groups">
	              <div class="antigravity-group-card">
	                <label for="agSelect_gemini">
	                  <span data-i18n="antigravityGroupGemini">Gemini 组</span>
	                  <code>gemini</code>
	                </label>
	                <select id="agSelect_gemini" class="antigravity-group-select" data-group="gemini" onchange="onAntigravityGroupChange()">
	                  <option value="" data-i18n="unselectedOption">— 未选择 —</option>
	                </select>
	              </div>
	              <div class="antigravity-group-card">
	                <label for="agSelect_claude_gpt">
	                  <span data-i18n="antigravityGroupClaudeGPT">Claude/GPT 组</span>
	                  <code>claude_gpt</code>
	                </label>
	                <select id="agSelect_claude_gpt" class="antigravity-group-select" data-group="claude_gpt" onchange="onAntigravityGroupChange()">
	                  <option value="" data-i18n="unselectedOption">— 未选择 —</option>
	                </select>
	              </div>
	              <div class="antigravity-group-card">
	                <label for="agSelect_gemini_4_argon">
	                  <span data-i18n="antigravityGroupArgon">Gemini 4 Argon 组</span>
	                  <code>gemini_4_argon</code>
	                </label>
	                <select id="agSelect_gemini_4_argon" class="antigravity-group-select" data-group="gemini_4_argon" onchange="onAntigravityGroupChange()">
	                  <option value="" data-i18n="unselectedOption">— 未选择 —</option>
	                </select>
	              </div>
	            </div>

	            <div class="config-preview-box">
	              <div class="preview-header">
	                <span class="preview-title" data-i18n="configPreview">配置预览 (只读)</span>
	                <button id="copyConfigButton" type="button" class="secondary preview-copy-btn" onclick="copyAntigravityConfig()" data-i18n="copyConfig">复制配置值</button>
	              </div>
	              <div class="preview-field">
	                <label for="previewModelsGroup"><code>activation_models.antigravity.models_group</code></label>
	                <input id="previewModelsGroup" type="text" readonly placeholder="例如: gemini|claude_gpt|gemini_4_argon" onclick="copyFieldValue(this)">
	              </div>
	              <div class="preview-field">
	                <label for="previewModels"><code>activation_models.antigravity.models</code></label>
	                <input id="previewModels" type="text" readonly placeholder="例如: gemini-2.5-pro|claude-3-7-sonnet|gemini-4-argon" onclick="copyFieldValue(this)">
	              </div>
	            </div>
	          </div>

	          <div class="actions">
	            <button id="refreshCredentialsButton" type="button" class="secondary" data-i18n="refresh" onclick="loadAuthFiles(this)">刷新凭证</button>
	            <button id="manualActivateButton" type="button" data-i18n="activate" onclick="triggerActivation(this)">手动触发唤醒</button>
	          </div>
	        </div>

	        <div id="recordsPanel" class="tab-panel">
	          <div class="records-header">
	            <div class="records-header-text">
	              <h3 data-i18n="recordsTitle">执行记录</h3>
	              <p class="records-hint" data-i18n="recordsHint">进程内最近 5 次（重启清空）</p>
	            </div>
	            <button id="refreshRecordsButton" type="button" class="secondary" data-i18n="refreshRecords" onclick="loadRunHistory(this)">刷新记录</button>
	          </div>
	          <div id="recordsList" class="records-list"></div>
	        </div>

	        <div id="helpPanel" class="tab-panel">
	          <div class="help-content">
	            <h3 data-i18n="helpConfigTitle">配置字段说明</h3>
	            <p data-i18n="helpConfigDesc">配置 quota-activation 插件所需的字段如下：</p>
	            <ul>
	              <li><code>auto_activate</code>: <span data-i18n="helpAutoActivate">是否开启自动配额唤醒。</span></li>
	              <li><code>enable_before_activation</code>: <span data-i18n="helpEnableBefore">是否在唤醒前自动启用已被禁用的凭证。</span></li>
	              <li><code data-optional="true">scan_interval</code>: <span data-i18n="helpScanInterval">可选；自动扫描间隔，单位分钟，填写纯数字即可（默认 30）。</span></li>
	              <li><code data-optional="true">activation_request_timeout</code>: <span data-i18n="helpActivationTimeout">可选；唤醒请求超时，单位秒，填写纯数字即可（默认 60）。</span></li>
	              <li><code data-optional="true">max_concurrency</code>: <span data-i18n="helpMaxConcurrency">可选；自动扫描与激活的并发上限（worker 池大小），默认 1。</span></li>
	              <li><code data-optional="true">activation_prompt</code>: <span data-i18n="helpActivationPrompt">可选；唤醒提示词。</span></li>
	            </ul>
	            <h3 data-i18n="helpActivationModelsTitle">自动唤醒模型字段</h3>
	            <p data-i18n="helpActivationModelsDesc">直接填写模型名称，不需要填写 JSON 对象：</p>
	            <ul>
	              <li><code>activation_models.codex.models</code>: <span data-i18n="helpCodexModels">Codex 自动唤醒模型名称，例如 <code>gpt-5-mini</code>。</span></li>
	              <li><code>activation_models.antigravity.models_group</code>: <span data-i18n="helpAntigravityGroup">Antigravity 自动唤醒模型组，可选 <code>gemini</code>、<code>claude_gpt</code> 或 <code>gemini_4_argon</code>。</span></li>
	              <li><code>activation_models.antigravity.models</code>: <span data-i18n="helpAntigravityModels">当前 Antigravity 模型组的模型名称，仍是所选的单个模型名。</span></li>
	            </ul>
	            <p data-i18n="helpAntigravityMultiGroupDesc">同一凭证支持同时选择多个模型组，生成固定顺序（<code>gemini|claude_gpt|gemini_4_argon</code>）的 <code>models_group</code> 与 <code>models</code> 预览。<code>|</code> 为位置配对符号，每组仅对应一个模型；旧单值配置完全兼容；不支持逗号或数组格式。Argon 模型保留在独立的 <code>gemini_4_argon</code> 组。</p>
	          </div>
	        </div>
	      </div>
	    </section>
	  </main>
	  <script>
	    const STATUS_PATH = "/v0/management/quota-reset/status";
	    const AUTH_FILES_PATH = "/v0/management/quota-reset/auth-files";
	    const AUTH_FILE_MODELS_PATH = "/v0/management/auth-files/models";
	    const ACTIVATE_PATH = "/v0/management/quota-reset/activate";
	    const DIAGNOSTICS_PATH = "/v0/management/quota-reset/diagnostics";
	    const translations = {
	      "zh-CN": {
	        activate: "手动触发唤醒",
	        activationDone: "手动唤醒请求已完成。",
	        activationFailed: "手动唤醒失败，请稍后重试或检查凭证状态。",
	        statusFailed: "唤醒失败",
	        activationRunning: "唤醒中...",
	        credential: "凭证",
	        disabled: "已禁用",
	        credentialDisabled: "请先启用凭证后再尝试唤醒",
	        loadingCredentials: "刷新中...",
	        keyFailed: "管理密钥验证失败，请检查后重试。",
	        keyPassed: "管理密钥验证通过。",
	        managementKeyPlaceholder: "输入 CPA 管理密钥",
	        missingCredential: "请先选择凭证。",
	        missingKey: "请先输入 CPA 管理密钥。",
	        missingModel: "请先选择已启用的唤醒模型。",
	        model: "唤醒模型",
	        noAuthFiles: "未发现可唤醒凭证。",
	        noModels: "该凭证没有启用的唤醒模型组。",
	        refresh: "刷新凭证",
	        statusBusy: "执行中",
	        statusSkipped: "已跳过",
	        statusSuccess: "成功",
	        statusUnknown: "未知",
	        title: "配额唤醒",
	        usable: "可用",
	        verifyKey: "验证密钥",
	        verifyingKey: "验证中...",
	        manualTab: "手动唤醒",
	        helpTab: "帮助",
	        searchPlaceholder: "搜索凭证...",
	        helpConfigTitle: "配置字段说明",
	        helpConfigDesc: "配置 quota-activation 插件所需的字段如下：",
	        helpActivationModelsTitle: "自动唤醒模型字段",
	        helpActivationModelsDesc: "直接填写模型名称，不需要填写 JSON 对象：",
	        helpAutoActivate: "是否开启自动配额唤醒。",
	        helpEnableBefore: "是否在唤醒前自动启用已被禁用的凭证。",
	        helpScanInterval: "可选；自动扫描间隔，单位分钟，填写纯数字即可（默认 30）。",
	        helpActivationTimeout: "可选；唤醒请求超时，单位秒，填写纯数字即可（默认 60）。",
	        helpMaxConcurrency: "可选；自动扫描与激活的并发上限（worker 池大小），默认 1。",
	        helpActivationPrompt: "可选；唤醒提示词。",
	        helpCodexModels: "Codex 自动唤醒模型名称，例如 gpt-5-mini。",
	        helpAntigravityGroup: "Antigravity 自动唤醒模型组，可选 gemini、claude_gpt 或 gemini_4_argon。",
	        helpAntigravityModels: "当前 Antigravity 模型组的模型名称，仍是所选的单个模型名。",
	        antigravityModelGroups: "Antigravity 模型组配置（每组最多选一个，可选 0..3 组）",
	        configPreview: "配置预览 (只读)",
	        copyConfig: "复制配置值",
	        copyConfigSuccess: "已复制配置到剪贴板",
	        unselectedOption: "— 未选择 —",
	        antigravityGroupGemini: "Gemini 组",
	        antigravityGroupClaudeGPT: "Claude/GPT 组",
	        antigravityGroupArgon: "Gemini 4 Argon 组",
	        helpAntigravityMultiGroupDesc: "同一凭证支持同时选择多个模型组，生成固定顺序（gemini|claude_gpt|gemini_4_argon）的 models_group 与 models 预览。| 为位置配对符号，每组仅对应一个模型；旧单值配置完全兼容；不支持逗号或数组格式。Argon 模型保留在独立的 gemini_4_argon 组。",
	        activationSummary: "手动唤醒完成：成功 {success}，失败 {failed}，跳过 {skipped}",
	        recordsTab: "执行记录",
	        recordsTitle: "执行记录",
	        recordsHint: "进程内最近 5 次（重启清空）",
	        refreshRecords: "刷新记录",
	        noRecords: "暂无执行记录",
	        noRecordsAutoOff: "自动唤醒未启用。请在配置中将 auto_activate 设为 true；开启后每次扫描会写入摘要，也可手动唤醒产生记录。",
	        triggerManual: "手动唤醒",
	        triggerAuto: "自动唤醒",
	        kindScan: "定时扫描",
	        kindActivationAuto: "自动唤醒",
	        kindActivationManual: "手动唤醒",
	        providerBreakdown: "按凭证提供商",
	        statAttempted: "尝试",
	        statSucceeded: "成功",
	        statFailed: "失败",
	        statSkipped: "跳过",
	        loadingRecords: "刷新中..."
	      },
	      "en-US": {
	        activate: "Trigger activation",
	        activationDone: "Manual activation request completed.",
	        activationFailed: "Manual activation failed. Try again later or check the credential status.",
	        statusFailed: "Activation failed",
	        activationRunning: "Activating...",
	        credential: "Credential",
	        disabled: "Disabled",
	        credentialDisabled: "Enable the credential before trying to wake it up.",
	        loadingCredentials: "Refreshing...",
	        keyFailed: "Management key verification failed. Check it and try again.",
	        keyPassed: "Management key verified.",
	        managementKeyPlaceholder: "Enter CPA management key",
	        missingCredential: "Select a credential first.",
	        missingKey: "Enter the CPA management key first.",
	        missingModel: "Select an enabled activation model first.",
	        model: "Activation model",
	        noAuthFiles: "No activatable credentials found.",
	        noModels: "This credential has no enabled activation model group.",
	        refresh: "Refresh credentials",
	        statusBusy: "Busy",
	        statusSkipped: "Skipped",
	        statusSuccess: "Success",
	        statusUnknown: "Unknown",
	        title: "Quota activation",
	        usable: "Available",
	        verifyKey: "Verify key",
	        verifyingKey: "Verifying...",
	        manualTab: "Manual Activation",
	        helpTab: "Help",
	        searchPlaceholder: "Search credentials...",
	        helpConfigTitle: "Configuration Fields",
	        helpConfigDesc: "The fields required to configure the quota-activation plugin are as follows:",
	        helpActivationModelsTitle: "Automatic activation model fields",
	        helpActivationModelsDesc: "Enter model names directly; no JSON object is required:",
	        helpAutoActivate: "Whether to enable automatic quota activation.",
	        helpEnableBefore: "Whether to automatically enable disabled credentials before activation.",
	        helpScanInterval: "Optional; automatic scan interval in minutes. Enter a number only (default 30).",
	        helpActivationTimeout: "Optional; activation request timeout in seconds. Enter a number only (default 60).",
	        helpMaxConcurrency: "Optional; concurrency limit for automatic scan activation (worker pool size); default 1.",
	        helpActivationPrompt: "Optional; activation prompt text.",
	        helpCodexModels: "Codex automatic activation model name, for example gpt-5-mini.",
	        helpAntigravityGroup: "Antigravity automatic activation model group: gemini, claude_gpt, or gemini_4_argon.",
	        helpAntigravityModels: "Model name for the current Antigravity model group (still the selected single model name).",
	        antigravityModelGroups: "Antigravity Model Groups (up to 1 per group, 0..3 groups selectable)",
	        configPreview: "Configuration Preview (Read-only)",
	        copyConfig: "Copy Config Values",
	        copyConfigSuccess: "Configuration copied to clipboard",
	        unselectedOption: "— None —",
	        antigravityGroupGemini: "Gemini Group",
	        antigravityGroupClaudeGPT: "Claude/GPT Group",
	        antigravityGroupArgon: "Gemini 4 Argon Group",
	        helpAntigravityMultiGroupDesc: "The same credential supports selecting multiple model groups simultaneously, generating '|'-separated models_group and models strings (fixed order: gemini|claude_gpt|gemini_4_argon). Unselected groups are omitted; single-group configs remain fully compatible; do not use arrays or commas.",
	        activationSummary: "Manual activation completed: {success} succeeded, {failed} failed, {skipped} skipped",
	        recordsTab: "Execution Records",
	        recordsTitle: "Execution Records",
	        recordsHint: "Last 5 runs in process (cleared on restart)",
	        refreshRecords: "Refresh records",
	        noRecords: "No execution records",
	        noRecordsAutoOff: "Auto activation is off. Set auto_activate to true in config; each scan tick will be recorded, and manual activation also writes records.",
	        triggerManual: "Manual activation",
	        triggerAuto: "Auto activation",
	        kindScan: "Scheduled scan",
	        kindActivationAuto: "Auto activation",
	        kindActivationManual: "Manual activation",
	        providerBreakdown: "By provider",
	        statAttempted: "Attempted",
	        statSucceeded: "Succeeded",
	        statFailed: "Failed",
	        statSkipped: "Skipped",
	        loadingRecords: "Refreshing...",
	        antigravityModelGroups: "Antigravity Model Groups (up to 1 model per group, 0..3 groups)",
	        configPreview: "Config Preview (Read-only)",
	        copyConfig: "Copy Config Values",
	        copyConfigSuccess: "Config copied to clipboard",
	        unselectedOption: "— Unselected —",
	        antigravityGroupGemini: "Gemini Group",
	        antigravityGroupClaudeGPT: "Claude/GPT Group",
	        antigravityGroupArgon: "Gemini 4 Argon Group",
	        helpAntigravityMultiGroupDesc: "Supports selecting multiple model groups simultaneously for the same credential, generating models_group and models previews in a fixed order (gemini|claude_gpt|gemini_4_argon). | is used for position-based pairing with exactly one model per group. Fully backward-compatible with legacy single-value configs. Comma and array formats are not supported. Argon models are retained in the dedicated gemini_4_argon group.",
	        activationSummary: "Manual activation completed: {success} succeeded, {failed} failed, {skipped} skipped"
	      }
	    };
	    const embeddedInDashboard = (() => {
	      try { return window.self !== window.top && window.parent.location.origin === window.location.origin && window.parent.location.pathname.startsWith("/dashboard/plugins/"); }
	      catch (_) { return false; }
	    })();
	    let language = "zh-CN";
	    let authFiles = [];
	    let activeProvider = "";
	    let toastTimer = 0;
	    function textFor(key) { return translations[language][key] || translations["zh-CN"][key] || key; }
	    function message(text) { const toast = document.getElementById("managementToast"); toast.textContent = text; if (toastTimer) { clearTimeout(toastTimer); } if (text) { toastTimer = setTimeout(() => { toast.textContent = ""; toastTimer = 0; }, 2500); } else { toastTimer = 0; } }
	    function managementKey() { const key = document.getElementById("managementKey").value.trim(); if (!key) { message(textFor("missingKey")); } return key; }
	    function applyLanguage() { document.documentElement.lang = language; document.querySelectorAll("[data-i18n]").forEach((item) => { item.textContent = textFor(item.dataset.i18n); }); document.querySelectorAll("[data-i18n-placeholder]").forEach((item) => { item.placeholder = textFor(item.dataset.i18nPlaceholder); }); renderAuthFiles(); renderRunHistory(); }
	    function switchLanguage() { language = language === "zh-CN" ? "en-US" : "zh-CN"; applyLanguage(); message(""); }
	    function shortError(defaultKey) { return textFor(defaultKey); }
	    function setButtonBusy(control, labelKey) { if (!control) { return null; } const state = { control, labelKey: control.dataset.i18n || "", text: control.textContent }; control.disabled = true; control.textContent = textFor(labelKey); return state; }
	    function restoreButton(state) { if (!state) { return; } state.control.disabled = false; state.control.textContent = state.labelKey ? textFor(state.labelKey) : state.text; }
	    function activationErrorMessage(error) { const raw = String(error && error.message ? error.message : error); try { const payload = JSON.parse(raw); return payload.last_error || payload.message || raw || textFor("activationFailed"); } catch (_) { return raw || textFor("activationFailed"); } }
	    async function managementFetch(path, options) { const headers = { "Content-Type": "application/json", ...((options && options.headers) || {}) }; if (!embeddedInDashboard) { const key = managementKey(); if (!key) { throw new Error(textFor("missingKey")); } headers.Authorization = "Bearer " + key; } const response = await fetch(path, { ...(options || {}), headers }); const text = await response.text(); if (!response.ok) { throw new Error(text || response.statusText); } return text ? JSON.parse(text) : {}; }
	    async function verifyManagementKey(button) { const buttonState = setButtonBusy(button, "verifyingKey"); try { await managementFetch(STATUS_PATH, { method: "GET" }); document.getElementById("loginGate").hidden = true; document.getElementById("appShell").hidden = false; await loadAuthFiles(); message(textFor("keyPassed")); } catch (error) { message(shortError("keyFailed")); } finally { restoreButton(buttonState); } }
	    function setManualActionBusy(busy, labelKey) {
	      const refreshBtn = document.getElementById("refreshCredentialsButton");
	      const activateBtn = document.getElementById("manualActivateButton");
	      const state = { refresh: null, activate: null };
	      if (busy) {
	        if (refreshBtn) {
	          state.refresh = { disabled: refreshBtn.disabled, text: refreshBtn.textContent, labelKey: refreshBtn.dataset.i18n || "refresh" };
	          refreshBtn.disabled = true;
	          if (labelKey) { refreshBtn.textContent = textFor(labelKey); }
	        }
	        if (activateBtn) {
	          state.activate = { disabled: activateBtn.disabled, text: activateBtn.textContent, labelKey: activateBtn.dataset.i18n || "activate" };
	          activateBtn.disabled = true;
	        }
	      }
	      return state;
	    }
	    function restoreManualActionBusy(state) {
	      if (!state) { return; }
	      const refreshBtn = document.getElementById("refreshCredentialsButton");
	      const activateBtn = document.getElementById("manualActivateButton");
	      if (refreshBtn && state.refresh) {
	        refreshBtn.disabled = Boolean(state.refresh.disabled);
	        refreshBtn.textContent = state.refresh.labelKey ? textFor(state.refresh.labelKey) : state.refresh.text;
	      }
	      if (activateBtn && state.activate) {
	        activateBtn.disabled = Boolean(state.activate.disabled);
	        activateBtn.textContent = state.activate.labelKey ? textFor(state.activate.labelKey) : state.activate.text;
	      }
	    }
	    async function loadAuthFiles(button) {
	      void button;
	      const manualBusy = setManualActionBusy(true, "loadingCredentials");
	      try {
	        const result = await managementFetch(AUTH_FILES_PATH, { method: "GET" });
	        if (!result) { return; }
	        authFiles = Array.isArray(result.files) ? result.files : [];
	        await loadCredentialModels();
	        renderAuthFiles();
	        syncModelOptions();
	        if (authFiles.length === 0) { message(textFor("noAuthFiles")); }
	      } catch (error) {
	        message(activationErrorMessage(error));
	      } finally {
	        restoreManualActionBusy(manualBusy);
	      }
	    }
	    async function loadCredentialModels() {
	      const providerCache = {};
	      const pendingByProvider = {};
	      for (const credential of authFiles) {
	        if (!credential) { continue; }
	        if (Array.isArray(credential.models) && credential.models.length > 0) { continue; }
	        const provider = String(credential.provider || "").toLowerCase();
	        const cacheKey = provider || ("name:" + String(credential.name || credential.auth_id || ""));
	        if (!credential.name && !providerCache[cacheKey]) { credential.models = Array.isArray(credential.models) ? credential.models : []; continue; }
	        try {
	          if (!Object.prototype.hasOwnProperty.call(providerCache, cacheKey)) {
	            if (!pendingByProvider[cacheKey]) {
	              const sampleName = credential.name || (authFiles.find((item) => item && item.name && String(item.provider || "").toLowerCase() === provider) || {}).name;
	              pendingByProvider[cacheKey] = sampleName
	                ? managementFetch(AUTH_FILE_MODELS_PATH + "?name=" + encodeURIComponent(sampleName), { method: "GET" }).then((result) => Array.isArray(result && result.models) ? result.models : []).catch(() => [])
	                : Promise.resolve([]);
	            }
	            providerCache[cacheKey] = await pendingByProvider[cacheKey];
	          }
	          credential.models = modelChoicesForCredential(credential, providerCache[cacheKey] || []);
	        } catch (_) {
	          credential.models = Array.isArray(credential.models) ? credential.models : [];
	        }
	      }
	    }
	    function modelChoicesForCredential(credential, models) {
	      const choices = [];
	      for (const item of models) {
	        const value = String((item && (item.id || item.name || item.value)) || "").trim();
	        if (!value) { continue; }
	        if (credential.provider === "antigravity") {
	          const group = (item && (item.group || item.Group)) || antigravityModelGroup(value);
	          if (!group) { continue; }
	          choices.push({ value, label: (item && item.label) || ((group === "gemini_4_argon" ? "Gemini 4 Argon" : (group === "gemini" ? "Gemini" : "Claude/GPT")) + " · " + value), group });
	        } else {
	          choices.push({ value, label: (item && item.label) || ("Codex · " + value) });
	        }
	      }
	      return choices;
	    }
	    function antigravityModelGroup(model) { const normalized = String(model || "").toLowerCase().replace(/[-_]/g, " ").replace(/\s+/g, " ").trim(); if (normalized.includes("gemini 4 argon")) { return "gemini_4_argon"; } if (normalized.includes("gemini")) { return "gemini"; } if (normalized.includes("claude") || normalized.includes("gpt")) { return "claude_gpt"; } return ""; }
	    function switchTab(tab) {
	      document.getElementById("manualTabButton").classList.toggle("active", tab === "manual");
	      document.getElementById("recordsTabButton").classList.toggle("active", tab === "records");
	      document.getElementById("helpTabButton").classList.toggle("active", tab === "help");
	      document.getElementById("manualPanel").classList.toggle("active", tab === "manual");
	      document.getElementById("recordsPanel").classList.toggle("active", tab === "records");
	      document.getElementById("helpPanel").classList.toggle("active", tab === "help");
	      if (tab === "records") {
	        loadRunHistory();
	      }
	    }
	    function providerLabel(key) {
	      if (key === "antigravity") return "Antigravity";
	      if (key === "codex") return "Codex";
	      return language === "zh-CN" ? "其他" : "Other";
	    }
	    function normalizeProviderKey(provider) {
	      const key = String(provider || "").toLowerCase();
	      if (key === "antigravity" || key === "codex") return key;
	      return "other";
	    }
	    function credentialProvider(file) {
	      return file ? file["provider"] : "";
	    }
	    function availableProviders() {
	      const seen = new Set();
	      const order = ["antigravity", "codex", "other"];
	      for (const file of authFiles) {
	        seen.add(normalizeProviderKey(credentialProvider(file)));
	      }
	      return order.filter((key) => seen.has(key));
	    }
	    function switchProvider(provider) {
	      activeProvider = provider || "";
	      renderAuthFiles();
	      syncModelOptions();
	    }
	    function renderProviderTabs(providers) {
	      const tabs = document.getElementById("providerTabs");
	      if (!tabs) { return; }
	      tabs.innerHTML = "";
	      for (const key of providers) {
	        const button = document.createElement("button");
	        button.type = "button";
	        button.className = "provider-tab" + (key === activeProvider ? " active" : "");
	        button.dataset.provider = key;
	        button.textContent = providerLabel(key);
	        button.onclick = () => switchProvider(key);
	        tabs.appendChild(button);
	      }
	    }
	    let selectedModel = null;
	    function renderAuthFiles() {
	      const list = document.getElementById("credentialSelect");
	      if (!list) { return; }
	      const providers = availableProviders();
	      if (!activeProvider || !providers.includes(activeProvider)) {
	        activeProvider = providers.length > 0 ? providers[0] : "";
	      }
	      renderProviderTabs(providers);
	      const searchVal = (document.getElementById("credentialSearch")?.value || "").toLowerCase().trim();
	      const selectedValues = Array.from(list.querySelectorAll('input[type="checkbox"]:checked')).map((input) => input.value);
	      list.innerHTML = "";
	      const filtered = authFiles.filter(file => {
	        if (normalizeProviderKey(credentialProvider(file)) !== activeProvider) return false;
	        if (!searchVal) return true;
	        const label = (file.label || "").toLowerCase();
	        const authId = (file.auth_id || "").toLowerCase();
	        return label.includes(searchVal) || authId.includes(searchVal);
	      });
	      for (const file of filtered) {
	        const row = document.createElement("label");
	        row.className = "credential-item";
	        row.classList.toggle("disabled", Boolean(file.disabled));
	        row.onclick = () => {
	          if (!file.disabled) { return; }
	          message(textFor("credentialDisabled"));
	        };
	        const checkbox = document.createElement("input");
	        checkbox.type = "checkbox";
	        checkbox.value = file.auth_id;
	        checkbox.disabled = Boolean(file.disabled);
	        checkbox.checked = !file.disabled && selectedValues.includes(file.auth_id);
	        checkbox.onchange = () => syncModelOptions();
	        const text = document.createElement("span");
	        text.textContent = [file.label || file.auth_id, file.disabled ? textFor("disabled") : textFor("usable")].filter(Boolean).join(" · ");
	        row.appendChild(checkbox);
	        row.appendChild(text);
	        list.appendChild(row);
	      }
	    }
	    function setModelMenuOpen(open) {
	      const panel = document.getElementById("modelMenuPanel");
	      const button = document.getElementById("modelMenuButton");
	      if (!panel || !button) { return; }
	      panel.classList.toggle("open", !!open);
	      button.setAttribute("aria-expanded", open ? "true" : "false");
	    }
	    function toggleModelMenu() {
	      const panel = document.getElementById("modelMenuPanel");
	      if (!panel) { return; }
	      setModelMenuOpen(!panel.classList.contains("open"));
	    }
	    function chooseModel(value, group, label) {
	      selectedModel = value ? { value, group: group || "", label: label || value } : null;
	      const modelLabel = document.getElementById("modelMenuLabel");
	      if (modelLabel) {
	        modelLabel.textContent = selectedModel ? selectedModel.label : "—";
	      }
	      const panel = document.getElementById("modelMenuPanel");
	      if (panel) {
	        panel.querySelectorAll(".model-option").forEach((btn) => {
	          btn.classList.toggle("active", !!(selectedModel && btn.dataset.value === selectedModel.value));
	        });
	      }
	      setModelMenuOpen(false);
	    }
	    const ANTIGRAVITY_GROUP_ORDER = ["gemini", "claude_gpt", "gemini_4_argon"];

	    function selectedAntigravityPairs() {
	      const pairs = [];
	      for (const group of ANTIGRAVITY_GROUP_ORDER) {
	        const el = document.getElementById("agSelect_" + group);
	        const val = el ? el.value.trim() : "";
	        if (val) {
	          pairs.push({ group, model: val });
	        }
	      }
	      return pairs;
	    }

	    function generateAntigravityPreview() {
	      const pairs = selectedAntigravityPairs();
	      const modelsGroup = pairs.map((p) => p.group).join("|");
	      const models = pairs.map((p) => p.model).join("|");
	      return { models_group: modelsGroup, models: models, pairs };
	    }

	    function updateAntigravityPreview() {
	      const preview = generateAntigravityPreview();
	      const groupInput = document.getElementById("previewModelsGroup");
	      const modelsInput = document.getElementById("previewModels");
	      if (groupInput) { groupInput.value = preview.models_group; }
	      if (modelsInput) { modelsInput.value = preview.models; }
	      return preview;
	    }

	    function onAntigravityGroupChange() {
	      updateAntigravityPreview();
	    }

	    function copyAntigravityConfig() {
	      const preview = updateAntigravityPreview();
	      const textToCopy = "models_group: " + preview.models_group + "\nmodels: " + preview.models;
	      if (navigator.clipboard && navigator.clipboard.writeText) {
	        navigator.clipboard.writeText(textToCopy).then(() => {
	          message(textFor("copyConfigSuccess"));
	        }).catch(() => {
	          message(preview.models_group ? (preview.models_group + " | " + preview.models) : textFor("noModels"));
	        });
	      } else {
	        message(preview.models_group ? (preview.models_group + " | " + preview.models) : textFor("noModels"));
	      }
	    }

	    function copyFieldValue(input) {
	      if (!input || !input.value) { return; }
	      input.select();
	      if (navigator.clipboard && navigator.clipboard.writeText) {
	        navigator.clipboard.writeText(input.value).then(() => {
	          message(textFor("copyConfigSuccess"));
	        }).catch(() => {});
	      }
	    }

	    function syncModelOptions() {
	      const isAntigravity = (activeProvider === "antigravity");
	      const singleSection = document.getElementById("singleModelSection");
	      const agSection = document.getElementById("antigravityModelSection");
	      if (singleSection) { singleSection.style.display = isAntigravity ? "none" : ""; }
	      if (agSection) { agSection.style.display = isAntigravity ? "" : "none"; }

	      const credentials = selectedCredentials();
	      const seen = new Set();
	      const choices = [];
	      for (const credential of credentials) {
	        const models = credential && Array.isArray(credential.models) ? credential.models : [];
	        for (const item of models) {
	          if (!seen.has(item.value)) {
	            seen.add(item.value);
	            choices.push(item);
	          }
	        }
	      }

	      if (isAntigravity) {
	        for (const group of ANTIGRAVITY_GROUP_ORDER) {
	          const select = document.getElementById("agSelect_" + group);
	          if (!select) { continue; }
	          const prevVal = select.value;
	          select.innerHTML = "";
	          const defaultOpt = document.createElement("option");
	          defaultOpt.value = "";
	          defaultOpt.textContent = textFor("unselectedOption");
	          select.appendChild(defaultOpt);

	          const groupChoices = choices.filter((item) => item.group === group);
	          for (const item of groupChoices) {
	            const opt = document.createElement("option");
	            opt.value = item.value;
	            opt.textContent = item.value;
	            select.appendChild(opt);
	          }

	          if (prevVal && groupChoices.some((item) => item.value === prevVal)) {
	            select.value = prevVal;
	          } else {
	            select.value = "";
	          }
	        }
	        updateAntigravityPreview();

	        if (credentials.length > 0 && choices.length === 0) {
	          const hasEnabled = credentials.some((credential) => credential && !credential.disabled);
	          if (hasEnabled) {
	            message(textFor("noModels"));
	          }
	        }
	      } else {
	        const panel = document.getElementById("modelMenuPanel");
	        if (!panel) { return; }
	        const selectedVal = selectedModel ? selectedModel.value : "";
	        panel.innerHTML = "";
	        for (const item of choices) {
	          const option = document.createElement("button");
	          option.type = "button";
	          option.className = "model-option" + (item.value === selectedVal ? " active" : "");
	          option.dataset.value = item.value;
	          option.dataset.group = item.group || "";
	          option.textContent = item.label || item.value;
	          option.onclick = () => { chooseModel(item.value, item.group || "", item.label || item.value); };
	          panel.appendChild(option);
	        }
	        if (choices.length === 0) {
	          chooseModel("", "", "—");
	        } else if (!choices.some((item) => item.value === selectedVal)) {
	          chooseModel(choices[0].value, choices[0].group || "", choices[0].label || choices[0].value);
	        } else {
	          const current = choices.find((item) => item.value === selectedVal);
	          chooseModel(current.value, current.group || "", current.label || current.value);
	        }
	        if (credentials.length > 0 && choices.length === 0) {
	          const hasEnabled = credentials.some((credential) => credential && !credential.disabled);
	          if (hasEnabled) {
	            message(textFor("noModels"));
	          }
	        }
	      }
	    }
	    function selectedCredentials() {
	      const list = document.getElementById("credentialSelect");
	      if (!list) return [];
	      const selectedValues = Array.from(list.querySelectorAll('input[type="checkbox"]:checked')).map((input) => input.value);
	      return authFiles.filter((item) => selectedValues.includes(item.auth_id));
	    }
	    function selectedCredential() {
	      const list = selectedCredentials();
	      return list.length > 0 ? list[0] : null;
	    }
	    function selectedModelOption() {
	      if (!selectedModel || !selectedModel.value) { return null; }
	      return { value: selectedModel.value, dataset: { group: selectedModel.group || "" } };
	    }
	    function quotaPayloadForActivation(credential, modelOption) { return credential ? (credential.quota_payload || {}) : {}; }
	    async function triggerActivation(button) {
	      const credentials = selectedCredentials();
	      if (credentials.length === 0) {
	        message(textFor("missingCredential"));
	        return;
	      }

	      const isAntigravity = (activeProvider === "antigravity");
	      let targets = [];
	      if (isAntigravity) {
	        const pairs = selectedAntigravityPairs();
	        if (pairs.length === 0) {
	          message(textFor("missingModel"));
	          return;
	        }
	        targets = pairs.map((p) => ({
	          model_group: p.group,
	          model: p.model,
	          option: { value: p.model, dataset: { group: p.group } }
	        }));
	      } else {
	        const modelOption = selectedModelOption();
	        if (!modelOption) {
	          message(textFor("missingModel"));
	          return;
	        }
	        targets = [{
	          model_group: modelOption.dataset.group || "",
	          model: modelOption.value,
	          option: modelOption
	        }];
	      }

	      const actBtn = button || document.getElementById("manualActivateButton");
	      const refBtn = document.getElementById("refreshCredentialsButton");
	      const actState = setButtonBusy(actBtn, "activationRunning");
	      let refBtnDisabledState = false;
	      if (refBtn) {
	        refBtnDisabledState = refBtn.disabled;
	        refBtn.disabled = true;
	      }
	      try {
	        let successCount = 0;
	        let failedCount = 0;
	        let skippedCount = 0;
	        let lastError = null;
	        for (const credential of credentials) {
	          for (const target of targets) {
	            const payload = {
	              auth_id: credential.auth_id,
	              provider: credential.provider,
	              model_group: target.model_group,
	              model: target.model,
	              disabled: Boolean(credential.disabled),
	              quota_payload: quotaPayloadForActivation(credential, target.option)
	            };
	            try {
	              const response = await managementFetch(ACTIVATE_PATH, {
	                method: "POST",
	                body: JSON.stringify(payload)
	              });
	              if (response && response.status === "success" && response.success === true) {
	                successCount++;
	              } else if (response && (response.status === "skipped" || response.status === "busy")) {
	                skippedCount++;
	                const errMsg = (response && response.last_error) ||
	                  (response.status === "skipped" ? textFor("statusSkipped") : textFor("statusBusy"));
	                lastError = new Error(errMsg);
	              } else {
	                failedCount++;
	                const errMsg = (response && response.last_error) ||
	                  (response && response.message ? response.message : textFor("activationFailed"));
	                lastError = new Error(errMsg);
	              }
	            } catch (error) {
	              failedCount++;
	              lastError = error;
	            }
	          }
	        }
	        if (failedCount === 0 && skippedCount === 0) {
	          message(textFor("activationDone"));
	        } else if (targets.length === 1 && credentials.length === 1 && lastError && (lastError.message === textFor("missingCredential") || lastError.message === textFor("missingModel"))) {
	          message(lastError.message);
	        } else {
	          const summaryTpl = textFor("activationSummary") || (language === "zh-CN" ? "手动唤醒完成：成功 {success}，失败 {failed}，跳过 {skipped}" : "Manual activation completed: {success} succeeded, {failed} failed, {skipped} skipped");
	          let summaryText = summaryTpl.replace("{success}", String(successCount)).replace("{failed}", String(failedCount)).replace("{skipped}", String(skippedCount));
	          if (lastError) {
	            summaryText += " (" + activationErrorMessage(lastError) + ")";
	          }
	          message(summaryText);
	        }
	      } catch (error) {
	        message(activationErrorMessage(error));
	      } finally {
	        restoreButton(actState);
	        if (refBtn) {
	          refBtn.disabled = refBtnDisabledState;
	        }
	      }
	    }
	    let runHistory = [];
	    let autoActivateEnabled = null;
	    async function loadRunHistory(button) {
	      const btnState = setButtonBusy(button, "loadingRecords");
	      try {
	        const result = await managementFetch(DIAGNOSTICS_PATH, { method: "GET" });
	        if (!result) { return; }
	        runHistory = Array.isArray(result.run_history) ? result.run_history : [];
	        autoActivateEnabled = result.config && typeof result.config.auto_activate === "boolean"
	          ? result.config.auto_activate
	          : null;
	        renderRunHistory();
	      } catch (error) {
	        message(activationErrorMessage(error));
	      } finally {
	        restoreButton(btnState);
	      }
	    }
	    function formatTime(isoStr) {
	      if (!isoStr) return "—";
	      try {
	        const d = new Date(isoStr);
	        if (isNaN(d.getTime())) return isoStr;
	        return d.toLocaleString(language === "zh-CN" ? "zh-CN" : "en-US", {
	          year: "numeric", month: "2-digit", day: "2-digit",
	          hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false
	        });
	      } catch (_) {
	        return isoStr;
	      }
	    }
	    function summarySeparator() { return language === "zh-CN" ? "：" : ": "; }
	    function kindLabel(item) {
	      const kind = String(item && item.kind || "").toLowerCase();
	      const trigger = String(item && item.trigger || "").toLowerCase();
	      // 合并后 auto tick 为 kind=scan trigger=auto；旧数据 kind=activation trigger=auto 仍兼容展示。
	      if (kind === "scan" || (kind === "activation" && trigger === "auto")) { return textFor("kindScan"); }
	      if (trigger === "auto") { return textFor("kindActivationAuto"); }
	      return textFor("kindActivationManual");
	    }
	    function kindBadgeClass(item) {
	      const kind = String(item && item.kind || "").toLowerCase();
	      const trigger = String(item && item.trigger || "").toLowerCase();
	      if (kind === "scan" || (kind === "activation" && trigger === "auto")) { return "scan"; }
	      return trigger === "auto" ? "auto" : "manual";
	    }
	    function localizeRecordMessage(raw) {
	      const msg = String(raw || "").trim();
	      if (!msg) { return ""; }
	      if (language !== "zh-CN") {
	        if (/状态保存被中断|状态保存失败/.test(msg)) {
	          return "Activation succeeded, but saving state was interrupted (internal timeout/cancel); credential is still valid";
	        }
	        if (msg.indexOf("调度器未选中目标凭证") >= 0) {
	          return "Activation scheduler did not select target credential";
	        }
	        if (/宿主模型执行器不可用/.test(msg) || /host model executor is unavailable/i.test(msg) || (/host_call_failed/i.test(msg) && /executor/i.test(msg))) {
	          return "Host model executor unavailable (host not ready or callback failed; not a credential failure)";
	        }
	        if (/定时扫描：/.test(msg)) {
	          return msg.replace("定时扫描：", "Scheduled scan: ").replace(/尝试 /g, "attempted ").replace(/成功 /g, "succeeded ").replace(/失败 /g, "failed ").replace(/跳过 /g, "skipped ").replace(/；跳过：/, "; skip: ").replace(/；失败：/, "; fail: ").replace(/本周期已处理/g, "cycle already processed").replace(/执行中/g, "busy").replace(/宿主模型执行器不可用（已冷却）/g, "host model executor unavailable (cooled down)");
	        }
	        const enMap = [
	          [/Codex唤醒失败：凭证已失效或鉴权失败（已重试3次）/, "Codex activation failed: credential invalid or unauthorized (retried 3 times)"],
	          [/Antigravity唤醒失败：凭证已失效或鉴权失败（已重试3次）/, "Antigravity activation failed: credential invalid or unauthorized (retried 3 times)"],
	          [/Codex唤醒失败：上游返回非成功状态（HTTP\s*(\d+)）/, "Codex activation failed: upstream returned a non-success status (HTTP $1)"],
	          [/Antigravity唤醒失败：上游返回非成功状态（HTTP\s*(\d+)）/, "Antigravity activation failed: upstream returned a non-success status (HTTP $1)"],
	          [/Codex唤醒失败：上游返回非成功状态/, "Codex activation failed: upstream returned a non-success status"],
	          [/Antigravity唤醒失败：上游返回非成功状态/, "Antigravity activation failed: upstream returned a non-success status"],
	          [/Codex唤醒失败：用量额度已耗尽/, "Codex activation failed: usage quota exhausted"],
	          [/Codex唤醒失败：响应体为空/, "Codex activation failed: empty response body"],
	          [/Antigravity唤醒失败：响应体为空/, "Antigravity activation failed: empty response body"],
	          [/Codex唤醒失败：响应不是合法数据/, "Codex activation failed: response is not valid data"],
	          [/Antigravity唤醒失败：响应不是合法数据/, "Antigravity activation failed: response is not valid data"],
	          [/Codex唤醒失败：响应缺少有效输出结构/, "Codex activation failed: response missing valid output structure"],
	          [/Codex唤醒失败：上游返回业务错误/, "Codex activation failed: upstream returned a business error"],
	          [/Antigravity唤醒失败：上游返回业务错误/, "Antigravity activation failed: upstream returned a business error"],
	          [/Codex唤醒失败：上游返回错误/, "Codex activation failed: upstream returned an error"],
	          [/Antigravity唤醒失败：响应缺少选项/, "Antigravity activation failed: response missing choices"],
	          [/Codex直连唤醒失败：无法读取凭证材料/, "Codex direct wake failed: unable to read credential material"],
	          [/Antigravity直连唤醒失败：无法读取凭证材料/, "Antigravity direct wake failed: unable to read credential material"],
	          [/本周期已唤醒，等待重置/, "Already woken this cycle; waiting for reset"],
	          [/额度周期已处理/, "Quota cycle already processed"],
	          [/读取物理凭证文件失败[：:]\s*(.*)/, "Failed to read physical credential file: $1"],
	          [/读取物理凭证文件失败/, "Failed to read physical credential file"],
	          [/读取凭证文件失败[：:]\s*(.*)/, "Failed to read credential file: $1"],
	          [/读取凭证文件失败/, "Failed to read credential file"],
	          [/直连唤醒失败[：:]\s*(.*)/, "Direct wake activation failed: $1"],
	          [/直连唤醒失败/, "Direct wake activation failed"],
	          [/列举凭证以解析可用模型失败[：:]\s*(.*)/, "Failed to list credentials for available models: $1"],
	          [/列举凭证以解析可用模型失败/, "Failed to list credentials for available models"],
	          [/列举凭证失败[：:]\s*(.*)/, "Failed to list credentials: $1"],
	          [/列举凭证失败/, "Failed to list credentials"],
	          [/缺少可用模型/, "Missing available models"],
	          [/网络连接失败[：:]\s*(.*)/, "Network connection failed: $1"],
	          [/网络连接失败/, "Network connection failed"],
	          [/^执行中$/, "In progress"],
	          [/唤醒请求无效[：:]\s*(.*)/, "Invalid activation request: $1"],
	          [/未找到匹配的凭证文件/, "Matching credential file not found"],
	          [/凭证已禁用/, "Credential disabled"],
	          [/解析凭证文件失败/, "Failed to decode credential file"],
	          [/写入凭证文件失败/, "Failed to write credential file"],
	          [/宿主模型执行失败/, "Host model execution failed"],
	          [/操作已取消/, "Operation canceled"],
	          [/操作超时/, "Operation timed out"],
	          [/提升凭证优先级失败/, "Failed to boost credential priority"],
	          [/科德克斯唤醒失败：/, "Codex activation failed: "],
	          [/反重力唤醒失败：/, "Antigravity activation failed: "]
	        ];
	        for (const pair of enMap) {
	          if (pair[0].test(msg)) {
	            return msg.replace(pair[0], pair[1]);
	          }
	        }
	        return msg;
	      }
	      const lower = msg.toLowerCase();
	      if (lower.indexOf("save activation state") >= 0 || lower.indexOf("save state context") >= 0 || (lower.indexOf("context canceled") >= 0 || lower.indexOf("context cancelled") >= 0)) {
	        return "唤醒已成功，但状态保存被中断（内部超时/取消），非凭证失效";
	      }
	      if (lower.indexOf("activation scheduler did not select target credential") >= 0) {
	        return "调度器未选中目标凭证";
	      }
	      if (lower.indexOf("host model executor is unavailable") >= 0 || (lower.indexOf("host_call_failed") >= 0 && lower.indexOf("executor") >= 0) || (lower.indexOf("host callback host.model.execute") >= 0 && lower.indexOf("unavailable") >= 0)) {
	        return "宿主模型执行器不可用（宿主未就绪或回调失败，非凭证失效）";
	      }
	      if (lower.indexOf("host model execute") >= 0 && lower.indexOf("unavailable") >= 0) {
	        return "宿主模型执行器不可用（宿主未就绪或回调失败，非凭证失效）";
	      }
	      if (/auto scan tick attempted=/.test(msg)) {
	        const m = msg.match(/attempted=(\d+)\s+succeeded=(\d+)\s+failed=(\d+)\s+skipped=(\d+)/i);
	        let out = m ? ("定时扫描：尝试 " + m[1] + " · 成功 " + m[2] + " · 失败 " + m[3] + " · 跳过 " + m[4]) : msg;
	        const skipPart = msg.split(/;\s*skip:\s*/i)[1];
	        if (skipPart) {
	          out += "；跳过：" + skipPart.replace(/quota 周期已处理/g, "本周期已处理").replace(/busy/gi, "执行中").replace(/×/g, " ×");
	        }
	        return out;
	      }
	      if (/^skip:\s*/i.test(msg)) {
	        return msg.replace(/^skip:\s*/i, "跳过：").replace(/quota 周期已处理/g, "本周期已处理");
	      }
	      return msg;
	    }
	    function isWarningMessage(msg) {
	      const text = String(msg || "");
	      if (!text) { return false; }
	      const lower = text.toLowerCase();
	      return /状态保存|非凭证失效/.test(text) || lower.indexOf("save activation state") >= 0 || lower.indexOf("context canceled") >= 0 || lower.indexOf("context cancelled") >= 0 || lower.indexOf("interrupted") >= 0;
	    }
	    function recordTone(item) {
	      const failed = Number(item && item.failed || 0);
	      const succeeded = Number(item && item.succeeded || 0);
	      const msg = localizeRecordMessage(item && item.message);
	      if (failed > 0 && succeeded === 0) { return "fail"; }
	      if (succeeded > 0 && (isWarningMessage(msg) || isWarningMessage(item && item.message))) { return "warn"; }
	      if (failed > 0 && succeeded > 0) { return "warn"; }
	      return "ok";
	    }
	    function providerDisplayName(name) {
	      const raw = String(name || "").toLowerCase();
	      if (raw === "antigravity") { return "Antigravity"; }
	      if (raw === "codex") { return "Codex"; }
	      if (raw === "xai" || raw === "x-ai") { return "xAI"; }
	      return name || providerLabel("other");
	    }
	    function statPill(label, value) {
	      const span = document.createElement("span");
	      span.className = "record-stat";
	      span.textContent = label + summarySeparator() + String(value == null ? 0 : value);
	      return span;
	    }
	    function renderRunHistory() {
	      const list = document.getElementById("recordsList");
	      if (!list) { return; }
	      list.innerHTML = "";
	      if (!Array.isArray(runHistory) || runHistory.length === 0) {
	        const empty = document.createElement("div");
	        empty.className = "empty-state";
	        empty.textContent = autoActivateEnabled === false ? textFor("noRecordsAutoOff") : textFor("noRecords");
	        list.appendChild(empty);
	        return;
	      }
	      for (const item of runHistory) {
	        const tone = recordTone(item);
	        const card = document.createElement("div");
	        card.className = "record-card" + (tone === "ok" ? "" : (" " + tone));
	        const head = document.createElement("div");
	        head.className = "record-head";
	        const meta = document.createElement("div");
	        meta.className = "record-meta";
	        const badge = document.createElement("span");
	        badge.className = "record-badge " + kindBadgeClass(item);
	        badge.textContent = kindLabel(item);
	        const timeSpan = document.createElement("span");
	        timeSpan.className = "record-time";
	        timeSpan.textContent = formatTime(item.at);
	        meta.appendChild(badge);
	        meta.appendChild(timeSpan);
	        head.appendChild(meta);
	        card.appendChild(head);
	        const stats = document.createElement("div");
	        stats.className = "record-stats";
	        stats.append(
	          statPill(textFor("statSucceeded"), item.succeeded || 0),
	          statPill(textFor("statFailed"), item.failed || 0),
	          statPill(textFor("statSkipped"), item.skipped || 0)
	        );
	        card.appendChild(stats);
	        if (Array.isArray(item.providers) && item.providers.length > 0) {
	          const title = document.createElement("div");
	          title.style.fontSize = "12px";
	          title.style.color = "#6b7280";
	          title.style.fontWeight = "650";
	          title.textContent = textFor("providerBreakdown");
	          card.appendChild(title);
	          const provDiv = document.createElement("div");
	          provDiv.className = "record-providers";
	          for (const p of item.providers) {
	            const row = document.createElement("div");
	            row.className = "record-provider";
	            const name = document.createElement("strong");
	            name.textContent = providerDisplayName(p.name);
	            row.appendChild(name);
	            const line = document.createElement("span");
	            line.textContent = [
	              textFor("statSucceeded") + summarySeparator() + (p.succeeded || 0),
	              textFor("statFailed") + summarySeparator() + (p.failed || 0),
	              textFor("statSkipped") + summarySeparator() + (p.skipped || 0)
	            ].join(" · ");
	            row.appendChild(line);
	            if (p.error) {
	              const err = document.createElement("span");
	              const mapped = localizeRecordMessage(p.error);
	              err.className = isWarningMessage(mapped) || isWarningMessage(p.error) ? "prov-warn" : "prov-err";
	              err.textContent = mapped;
	              row.appendChild(err);
	            }
	            provDiv.appendChild(row);
	          }
	          card.appendChild(provDiv);
	        }
	        list.appendChild(card);
	      }
	    }
	    document.addEventListener("click", (event) => {
	      const menu = document.getElementById("modelSelect");
	      if (menu && !menu.contains(event.target)) {
	        setModelMenuOpen(false);
	      }
	    });
	    applyLanguage();
	    if (embeddedInDashboard) {
	      document.getElementById("loginGate").hidden = true;
	      document.getElementById("appShell").hidden = false;
	      loadAuthFiles();
	    }
	  </script>
</body>
</html>`))
}
