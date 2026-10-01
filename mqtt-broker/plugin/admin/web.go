// 文件用途：内置管理页（登录页 + dashboard）的路由注册与 HTML/脚本渲染。
// 核心逻辑：adminUIRoutes 是管理页路由的单一事实来源，既用于注册 handler，
// 也用于让共享密钥中间件放行这些自带会话校验的页面路由（见 http_auth.go）。
// 关键注意事项：会话/CSRF 逻辑在 web_auth.go 与 session.go；dashboard 通过
// <meta name="csrf-token"> 拿到 CSRF 令牌，并由 fetch 包装器自动附带 X-CSRF-Token。

package admin

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

const (
	sessionCookieName     = "gmqtt_admin_session"
	loginErrorParam       = "error"
	loginErrorCredentials = "credentials"
	loginErrorForm        = "form"
	loginErrorRequired    = "unauthorized"
	loginErrorExpired     = "expired"
	loginErrorThrottled   = "throttled"
)

// adminUIRoute 描述一条管理页路由。
type adminUIRoute struct {
	method  string
	path    string
	handler func(a *Admin) runtime.HandlerFunc
}

// adminUIRoutes 列出全部管理页路由；它们各自完成会话/nonce/CSRF 校验，
// 因而在共享密钥中间件中被精确（方法 + 路径）放行，否则启用 http_auth_secret 后
// 浏览器无法登录 dashboard。
var adminUIRoutes = []adminUIRoute{
	{method: http.MethodGet, path: "/", handler: func(a *Admin) runtime.HandlerFunc { return a.serveLoginPage }},
	{method: http.MethodGet, path: "/dashboard", handler: func(a *Admin) runtime.HandlerFunc { return a.serveDashboardPage }},
	{method: http.MethodPost, path: "/login", handler: func(a *Admin) runtime.HandlerFunc { return a.handleLogin }},
	{method: http.MethodPost, path: "/logout", handler: func(a *Admin) runtime.HandlerFunc { return a.handleLogout }},
}

// isAdminUIRoute 报告请求是否精确命中管理页路由。
func isAdminUIRoute(r *http.Request) bool {
	for _, route := range adminUIRoutes {
		if r.Method == route.method && r.URL.Path == route.path {
			return true
		}
	}
	return false
}

func (a *Admin) registerAdminUI(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
	_ = ctx
	_ = endpoint
	_ = opts

	for _, route := range adminUIRoutes {
		if err := handleStaticPath(mux, route.method, route.path, route.handler(a)); err != nil {
			return err
		}
	}
	return nil
}

func handleStaticPath(mux *runtime.ServeMux, method, path string, h runtime.HandlerFunc) error {
	if mux == nil {
		return fmt.Errorf("nil mux")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path %q must start with '/'", path)
	}

	trimmed := strings.TrimPrefix(path, "/")
	var segments []string
	if trimmed == "" {
		segments = []string{""}
	} else {
		segments = strings.Split(trimmed, "/")
	}

	ops := make([]int, 0, len(segments)*2)
	pool := make([]string, len(segments))
	for i, segment := range segments {
		pool[i] = segment
		ops = append(ops, 2, i)
	}

	pattern, err := runtime.NewPattern(1, ops, pool, "", runtime.AssumeColonVerbOpt(true))
	if err != nil {
		return err
	}

	mux.Handle(method, pattern, h)
	return nil
}

// setAdminPageHeaders 为管理页设置内容类型与防缓存、防点击劫持响应头；
// 页面内嵌 CSRF 令牌 / 登录 nonce，不能被中间缓存或嵌入第三方 frame。
func setAdminPageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
}

func writeLoginPage(w http.ResponseWriter, errorCode, loginNonce string) {
	setAdminPageHeaders(w)
	var message string
	switch errorCode {
	case loginErrorCredentials:
		message = "Invalid username or password."
	case loginErrorForm:
		message = "Unable to parse the submitted form."
	case loginErrorRequired:
		message = "Please sign in to continue."
	case loginErrorExpired:
		message = "The sign-in form expired. Please try again."
	case loginErrorThrottled:
		message = "Too many failed attempts. Please try again later."
	}

	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>GMQTT Admin Login</title>
<style>
	body { font-family: Arial, sans-serif; background: #0b1220; color: #f5f6f8; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
	.container { background: #141d2f; padding: 32px 40px; border-radius: 12px; box-shadow: 0 12px 32px rgba(0,0,0,0.45); width: 320px; }
	.container h1 { margin-top: 0; font-size: 24px; text-align: center; }
	.form-group { margin-bottom: 16px; }
	label { display: block; margin-bottom: 6px; font-weight: bold; }
	input { width: 100%; padding: 10px; border: 1px solid #2a3652; border-radius: 6px; background: #0f172a; color: #f5f6f8; }
	button { width: 100%; padding: 12px; border: none; border-radius: 6px; background: #3b82f6; color: #ffffff; font-size: 16px; cursor: pointer; }
	button:hover { background: #2563eb; }
	.error { background: rgba(239,68,68,0.15); border: 1px solid #ef4444; color: #fca5a5; padding: 10px; border-radius: 6px; margin-bottom: 16px; text-align: center; }
</style>
</head>
<body>
<div class="container">
	<h1>GMQTT Admin</h1>`))
	if message != "" {
		_, _ = w.Write([]byte(`<div class="error">` + message + `</div>`))
	}
	_, _ = w.Write([]byte(`
	<form method="post" action="/login">
		<input type="hidden" name="` + loginNonceFormField + `" value="` + html.EscapeString(loginNonce) + `">
		<div class="form-group">
			<label for="username">Username</label>
			<input id="username" name="username" type="text" autocomplete="username" required>
		</div>
		<div class="form-group">
			<label for="password">Password</label>
			<input id="password" name="password" type="password" autocomplete="current-password" required>
		</div>
		<button type="submit">Sign in</button>
	</form>
</div>
</body>
</html>`))
}

func writeDashboardPage(w http.ResponseWriter, csrfToken string) {
	setAdminPageHeaders(w)
	writeDashboardFragments(w,
		dashboardDocumentStart(csrfToken),
		dashboardStyles(),
		dashboardBodyMarkup(csrfToken),
		dashboardScripts(),
		dashboardDocumentEnd(),
	)
}

func writeDashboardFragments(w http.ResponseWriter, fragments ...string) {
	for _, fragment := range fragments {
		_, _ = w.Write([]byte(fragment))
	}
}

func dashboardDocumentStart(csrfToken string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="csrf-token" content="` + html.EscapeString(csrfToken) + `">
<title>GMQTT 管理控制台</title>
<style>
`
}

func dashboardStyles() string {
	return `:root { color-scheme: dark; }
* { box-sizing: border-box; }
body { font-family: Arial, sans-serif; background: #0e1628; color: #f5f6f8; margin: 0; }
header { display: flex; justify-content: space-between; align-items: center; padding: 20px 32px; background: #101a32; box-shadow: 0 4px 20px rgba(0,0,0,0.35); }
h1 { margin: 0; font-size: 24px; }
main { padding: 24px 32px; display: grid; gap: 24px; }
@media (min-width: 1100px) {
	main { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
.panel { background: #13213d; border-radius: 12px; padding: 24px; box-shadow: 0 8px 32px rgba(0,0,0,0.3); display: flex; flex-direction: column; gap: 16px; }
.panel-header { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: flex-start; gap: 12px; }
.panel-header h2 { margin: 0; font-size: 20px; }
.panel-subtitle { margin: 4px 0 0; color: #94a3b8; font-size: 14px; }
.btn { border: none; border-radius: 6px; padding: 10px 18px; font-size: 14px; cursor: pointer; transition: background 0.2s ease, color 0.2s ease, border 0.2s ease; }
.btn-primary { background: #3b82f6; color: #ffffff; }
.btn-primary:hover { background: #2563eb; }
.btn-secondary { background: transparent; border: 1px solid #3b82f6; color: #3b82f6; }
.btn-secondary:hover { background: rgba(59,130,246,0.15); }
.table-container { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; font-size: 14px; }
thead { background: rgba(15,23,42,0.7); }
th, td { padding: 10px 12px; text-align: left; border-bottom: 1px solid rgba(148,163,184,0.15); }
tbody tr:hover { background: rgba(15,23,42,0.45); }
.status { font-size: 13px; color: #93c5fd; min-height: 18px; }
.status.error { color: #fca5a5; }
.status.success { color: #86efac; }
.form-grid { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); }
.form-grid label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: #d1d5db; }
.form-grid label.inline { flex-direction: row; align-items: center; }
.form-grid label.inline input { width: auto; }
.form-grid input, .form-grid select, .form-grid textarea { background: #0f172a; color: #f5f6f8; border: 1px solid #1f2a44; border-radius: 6px; padding: 8px 10px; font-size: 14px; }
textarea { resize: vertical; min-height: 96px; }
.result { background: #0f172a; padding: 16px; border-radius: 8px; font-family: "Fira Code", monospace; font-size: 13px; overflow-x: auto; margin: 0; }
.empty-row td { text-align: center; color: #94a3b8; font-style: italic; }
form .actions { display: flex; gap: 12px; align-items: center; }
.badge { display: inline-flex; align-items: center; background: rgba(59,130,246,0.15); border: 1px solid rgba(59,130,246,0.4); color: #93c5fd; font-size: 12px; border-radius: 999px; padding: 4px 10px; }
`
}

func dashboardBodyMarkup(csrfToken string) string {
	return strings.Join([]string{
		dashboardBodyStart(csrfToken),
		dashboardClientsPanelMarkup(),
		dashboardSubscriptionsPanelMarkup(),
		dashboardPublishPanelMarkup(),
		dashboardBodyEnd(),
	}, "")
}

func dashboardBodyStart(csrfToken string) string {
	return `</style>
</head>
<body>
<header>
	<h1>GMQTT 管理控制台</h1>
	<form method="post" action="/logout">
		<input type="hidden" name="` + csrfFormField + `" value="` + html.EscapeString(csrfToken) + `">
		<button class="btn btn-secondary" type="submit">退出登录</button>
	</form>
</header>
<main>
`
}

func dashboardClientsPanelMarkup() string {
	return `	<section class="panel" id="clients-panel">
		<div class="panel-header">
			<div>
				<h2>在线客户端</h2>
				<p class="panel-subtitle">查看当前接入 Broker 的客户端信息。</p>
			</div>
			<button class="btn btn-primary" id="clients-refresh" type="button">刷新</button>
		</div>
		<div class="status" id="clients-status"></div>
		<div class="table-container">
			<table>
				<thead>
					<tr>
						<th>客户端 ID</th>
						<th>用户名</th>
						<th>远程地址</th>
						<th>协议版本</th>
						<th>连接时间</th>
					</tr>
				</thead>
				<tbody id="clients-body">
					<tr class="empty-row"><td colspan="5">尚未加载数据</td></tr>
				</tbody>
			</table>
		</div>
	</section>
`
}

func dashboardSubscriptionsPanelMarkup() string {
	return `	<section class="panel" id="subscriptions-panel">
		<div class="panel-header">
			<div>
				<h2>过滤订阅</h2>
				<p class="panel-subtitle">根据类型、匹配规则、主题或客户端过滤订阅。</p>
			</div>
			<span class="badge">GET /v1/filter_subscriptions</span>
		</div>
		<form id="subscriptions-form" autocomplete="off">
			<div class="form-grid">
				<label>
					客户端 ID
					<input name="client_id" placeholder="可选">
				</label>
				<label>
					过滤类型
					<input name="filter_type" placeholder="例如 1,2,3">
				</label>
				<label>
					匹配类型
					<select name="match_type">
						<option value="">不限</option>
						<option value="1">主题名称相等 (1)</option>
						<option value="2">主题过滤匹配 (2)</option>
					</select>
				</label>
				<label>
					主题名称
					<input name="topic_name" placeholder="/a/b/+">
				</label>
				<label>
					返回数量
					<input name="limit" type="number" min="1" step="1" placeholder="默认">
				</label>
			</div>
			<div class="actions">
				<button class="btn btn-primary" type="submit">开始过滤</button>
				<div class="status" id="subscriptions-status"></div>
			</div>
		</form>
		<pre class="result" id="subscriptions-result">// 过滤结果将显示在这里</pre>
	</section>
`
}

func dashboardPublishPanelMarkup() string {
	return `	<section class="panel" id="publish-panel">
		<div class="panel-header">
			<div>
				<h2>发布消息</h2>
				<p class="panel-subtitle">通过管理 API 将 MQTT 消息发送到 Broker。</p>
			</div>
			<span class="badge">POST /v1/publish</span>
		</div>
		<form id="publish-form" autocomplete="off">
			<div class="form-grid">
				<label>
					主题名称
					<input name="topic_name" required placeholder="devices/+/data">
				</label>
				<label>
					消息内容
					<textarea name="payload" required placeholder="JSON / 文本 / Base64 字符串"></textarea>
				</label>
				<label>
					服务质量 (QoS)
					<select name="qos" required>
						<option value="0">0 - 最多一次</option>
						<option value="1">1 - 至少一次</option>
						<option value="2">2 - 刚好一次</option>
					</select>
				</label>
				<label class="inline">
					<input type="checkbox" name="retained">
					保留消息
				</label>
				<label>
					响应主题
					<input name="response_topic" placeholder="可选">
				</label>
				<label>
					Content Type
					<input name="content_type" placeholder="可选">
				</label>
				<label>
					关联数据
					<input name="correlation_data" placeholder="可选">
				</label>
				<label>
					消息过期时间 (秒)
					<input name="message_expiry" type="number" min="0" step="1" placeholder="可选">
				</label>
				<label>
					负载格式
					<select name="payload_format">
						<option value="">自动</option>
						<option value="0">0 - 未指定</option>
						<option value="1">1 - UTF-8</option>
					</select>
				</label>
			</div>
			<div class="actions">
				<button class="btn btn-primary" type="submit">发布</button>
				<div class="status" id="publish-status"></div>
			</div>
		</form>
		<pre class="result" id="publish-result">// 发布结果将在这里显示</pre>
	</section>
`
}

func dashboardBodyEnd() string {
	return `</main>
`
}

func dashboardScripts() string {
	return strings.Join([]string{
		dashboardFetchScript(),
		dashboardClientsScript(),
		dashboardSubscriptionsScript(),
		dashboardPublishScript(),
	}, "")
}

// dashboardFetchScript 包装 window.fetch：所有同源请求自动附带 X-CSRF-Token 与会话 cookie。
// 后续脚本中的 fetch 调用无需逐个修改即可满足服务端 CSRF 校验。
func dashboardFetchScript() string {
	return `<script>
(function() {
	const meta = document.querySelector('meta[name="csrf-token"]');
	const csrfToken = meta ? meta.getAttribute("content") : "";
	const nativeFetch = window.fetch.bind(window);
	window.fetch = function(input, init) {
		const options = Object.assign({}, init || {});
		const headers = new Headers(options.headers || {});
		if (csrfToken && !headers.has("` + CSRFHeader + `")) {
			headers.set("` + CSRFHeader + `", csrfToken);
		}
		options.headers = headers;
		options.credentials = "same-origin";
		return nativeFetch(input, options);
	};
})();
</script>
`
}

func dashboardClientsScript() string {
	return `<script>
const clientsRefreshBtn = document.getElementById("clients-refresh");
const clientsStatus = document.getElementById("clients-status");
const clientsBody = document.getElementById("clients-body");

async function loadClients() {
	clientsRefreshBtn.disabled = true;
	clientsRefreshBtn.textContent = "加载中...";
	clientsStatus.className = "status";
	clientsStatus.textContent = "正在获取客户端信息...";
	try {
		const res = await fetch("/v1/clients");
		if (!res.ok) {
			const text = await res.text();
			throw new Error(text || "请求失败");
		}
		const data = await res.json();
		const clients = Array.isArray(data.clients) ? data.clients : [];
		renderClients(clients);
		clientsStatus.className = "status success";
		clientsStatus.textContent = "已加载 " + clients.length + " 个客户端";
	} catch (err) {
		clientsStatus.className = "status error";
		clientsStatus.textContent = err.message;
		clientsBody.innerHTML = '<tr class="empty-row"><td colspan="5">无法加载数据</td></tr>';
	} finally {
		clientsRefreshBtn.disabled = false;
		clientsRefreshBtn.textContent = "刷新";
	}
}

function renderClients(clients) {
	if (!clients.length) {
		clientsBody.innerHTML = '<tr class="empty-row"><td colspan="5">暂无客户端连接</td></tr>';
		return;
	}
	const rows = clients.map(function(client) {
		const connectedAt = client.connected_at ? new Date(client.connected_at).toLocaleString() : "—";
		return "<tr>" +
			"<td>" + escapeHtml(client.client_id || "") + "</td>" +
			"<td>" + escapeHtml(client.username || "") + "</td>" +
			"<td>" + escapeHtml(client.remote_addr || "") + "</td>" +
			"<td>" + (client.version != null ? client.version : "") + "</td>" +
			"<td>" + escapeHtml(connectedAt) + "</td>" +
		"</tr>";
	});
	clientsBody.innerHTML = rows.join("");
}

function escapeHtml(value) {
	return String(value).replace(/[&<>"']/g, function(match) {
		switch (match) {
		case "&": return "&amp;";
		case "<": return "&lt;";
		case ">": return "&gt;";
		case '"': return "&quot;";
		case "'": return "&#39;";
		default: return match;
		}
	});
}

clientsRefreshBtn.addEventListener("click", loadClients);
loadClients();

`
}

func dashboardSubscriptionsScript() string {
	return `const subscriptionsForm = document.getElementById("subscriptions-form");
const subscriptionsStatus = document.getElementById("subscriptions-status");
const subscriptionsResult = document.getElementById("subscriptions-result");

subscriptionsForm.addEventListener("submit", async function(event) {
	event.preventDefault();
	subscriptionsStatus.className = "status";
	subscriptionsStatus.textContent = "正在过滤...";
	subscriptionsResult.textContent = "";
	const formData = new FormData(subscriptionsForm);
	const filterType = (formData.get("filter_type") || "").trim();
	const matchType = (formData.get("match_type") || "").trim();
	const topicName = (formData.get("topic_name") || "").trim();
	if (filterType && !matchType) {
		subscriptionsStatus.className = "status error";
		subscriptionsStatus.textContent = "当设置过滤类型时必须指定匹配类型";
		return;
	}
	if (matchType && !topicName) {
		subscriptionsStatus.className = "status error";
		subscriptionsStatus.textContent = "选择匹配类型时必须填写主题名称";
		return;
	}
	const params = new URLSearchParams();
	for (const [key, value] of formData.entries()) {
		if (!value) {
			continue;
		}
		params.append(key, value.toString().trim());
	}
	const url = "/v1/filter_subscriptions" + (params.toString() ? "?" + params.toString() : "");
	try {
		const res = await fetch(url);
		if (!res.ok) {
			const text = await res.text();
			throw new Error(text || "请求失败");
		}
		const data = await res.json();
		subscriptionsStatus.className = "status success";
		const count = Array.isArray(data.subscriptions) ? data.subscriptions.length : 0;
		subscriptionsStatus.textContent = "共找到 " + count + " 条订阅";
		subscriptionsResult.textContent = JSON.stringify(data, null, 2);
	} catch (err) {
		subscriptionsStatus.className = "status error";
		subscriptionsStatus.textContent = err.message;
		subscriptionsResult.textContent = "// 错误：" + err.message;
	}
});

`
}

func dashboardPublishScript() string {
	return `const publishForm = document.getElementById("publish-form");
const publishStatus = document.getElementById("publish-status");
const publishResult = document.getElementById("publish-result");

publishForm.addEventListener("submit", async function(event) {
	event.preventDefault();
	publishStatus.className = "status";
	publishStatus.textContent = "正在发布...";
	publishResult.textContent = "";
	const formData = new FormData(publishForm);
	const topicName = (formData.get("topic_name") || "").trim();
	const payload = (formData.get("payload") || "").trim();
	const qos = formData.get("qos");
	if (!topicName || !payload || qos == null) {
		publishStatus.className = "status error";
		publishStatus.textContent = "必须填写主题、消息内容和 QoS";
		return;
	}
	const body = {
		topic_name: topicName,
		payload: payload,
		qos: Number(qos),
		retained: formData.get("retained") === "on",
	};
	const optionalFields = ["response_topic", "content_type", "correlation_data"];
	optionalFields.forEach(function(field) {
		const val = (formData.get(field) || "").trim();
		if (val) {
			body[field] = val;
		}
	});
	const messageExpiry = (formData.get("message_expiry") || "").trim();
	if (messageExpiry) {
		body.message_expiry = Number(messageExpiry);
	}
	const payloadFormat = (formData.get("payload_format") || "").trim();
	if (payloadFormat) {
		body.payload_format = Number(payloadFormat);
	}
	try {
		const res = await fetch("/v1/publish", {
			method: "POST",
			headers: {
				"Content-Type": "application/json"
			},
			body: JSON.stringify(body)
		});
		if (!res.ok) {
			const text = await res.text();
			throw new Error(text || "发布失败");
		}
		let text = await res.text();
		let data = null;
		try {
			data = text ? JSON.parse(text) : {};
		} catch (parseError) {
			data = { raw: text };
		}
		publishStatus.className = "status success";
		publishStatus.textContent = "发布请求已接受";
		publishResult.textContent = JSON.stringify(data, null, 2);
	} catch (err) {
		publishStatus.className = "status error";
		publishStatus.textContent = err.message;
		publishResult.textContent = "// 错误：" + err.message;
	}
});
</script>
`
}

func dashboardDocumentEnd() string {
	return `</body>
</html>`
}
