package prometheus

import (
	"net/http"
	"strings"
)

const dashboardPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>GMQTT Metrics Dashboard</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root { color-scheme: dark; font-family: Arial, sans-serif; background: #0b1526; color: #e5e7eb; }
body { margin: 0; background: #0b1526; }
header { padding: 18px 20px; background: #0f172a; border-bottom: 1px solid #243244; position: sticky; top: 0; }
h1 { margin: 0; font-size: 18px; }
main { padding: 16px 20px 28px; display: grid; gap: 14px; }
.bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.status { font-size: 12px; color: #94a3b8; min-height: 18px; }
.status.error { color: #fca5a5; }
.status.success { color: #86efac; }
.status.loading { color: #93c5fd; }
.grid { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
.card, .panel, details { background: #0f172a; border: 1px solid #243244; border-radius: 8px; padding: 14px; }
h2, h3 { margin: 0 0 10px; font-size: 14px; color: #e2e8f0; }
.kv { display: grid; grid-template-columns: 1fr auto; gap: 6px 10px; font-size: 13px; }
.kv dt { color: #94a3b8; }
.kv dd { margin: 0; font-variant-numeric: tabular-nums; color: #f8fafc; }
table { width: 100%; border-collapse: collapse; font-size: 12px; }
th, td { padding: 8px 10px; border-bottom: 1px solid #243244; text-align: left; font-variant-numeric: tabular-nums; }
thead th { background: #1e293b; color: #cbd5e1; }
summary { cursor: pointer; color: #cbd5e1; }
pre { margin: 12px -14px -14px; background: #020617; padding: 12px; border-top: 1px solid #243244; color: #cbd5e1; font-size: 12px; line-height: 1.5; overflow-x: auto; }
code { font-family: Consolas, "Courier New", monospace; }
</style>
</head>
<body>
<header>
	<h1>GMQTT Metrics Dashboard</h1>
	<p>Source <code>__METRICS_PATH__</code>, refreshed every 5 seconds.</p>
</header>
<main>
	<div class="bar">
		<span id="status" class="status">Waiting for refresh...</span>
		<span>GET <code>__METRICS_PATH__</code></span>
	</div>
	<section class="grid">
		<article class="card">
			<h3>Broker Overview</h3>
			<dl class="kv">
				<dt>Connected total</dt><dd id="m_clients_connected">--</dd>
				<dt>Disconnected total</dt><dd id="m_clients_disconnected">--</dd>
				<dt>Active sessions</dt><dd id="m_sessions_active">--</dd>
				<dt>Inactive sessions</dt><dd id="m_sessions_inactive">--</dd>
				<dt>Current subscriptions</dt><dd id="m_sub_current">--</dd>
				<dt>Total subscriptions</dt><dd id="m_sub_total">--</dd>
				<dt>Queued messages</dt><dd id="m_msg_queued">--</dd>
				<dt>Dropped messages</dt><dd id="m_msg_dropped">--</dd>
			</dl>
		</article>
		<article class="card">
			<h3>Runtime</h3>
			<dl class="kv">
				<dt>Go version</dt><dd id="m_go_version">--</dd>
				<dt>Goroutines</dt><dd id="m_go_goroutines">--</dd>
				<dt>OS threads</dt><dd id="m_go_threads">--</dd>
				<dt>CPU seconds</dt><dd id="m_proc_cpu">--</dd>
				<dt>Resident memory</dt><dd id="m_proc_rss">--</dd>
				<dt>Open FDs</dt><dd id="m_proc_fds">--</dd>
			</dl>
		</article>
		<article class="card">
			<h3>PromHTTP</h3>
			<dl class="kv">
				<dt>200</dt><dd id="m_ph_200">--</dd>
				<dt>500</dt><dd id="m_ph_500">--</dd>
				<dt>503</dt><dd id="m_ph_503">--</dd>
				<dt>In flight</dt><dd id="m_ph_inflight">--</dd>
			</dl>
		</article>
	</section>
	<section class="panel">
		<h2>Message throughput by QoS</h2>
		<table><thead><tr><th>QoS</th><th>Received</th><th>Sent</th><th>Dropped</th></tr></thead><tbody id="qos-body"></tbody></table>
	</section>
	<section class="panel">
		<h2>MQTT packet statistics</h2>
		<table><thead><tr><th>Type</th><th>Received count</th><th>Received bytes</th><th>Sent count</th><th>Sent bytes</th></tr></thead><tbody id="pkt-body"></tbody></table>
	</section>
	<details><summary>Raw metrics</summary><pre id="raw-output">// loading...</pre></details>
</main>
<script>
const metricsPath = "__METRICS_PATH__";
const statusEl = document.getElementById("status");
function formatNumber(value, digits) {
	if (value === null || value === undefined || Number.isNaN(value)) return "--";
	if (digits !== undefined) return Number(value).toLocaleString(undefined, { maximumFractionDigits: digits });
	return Number(value).toLocaleString();
}
function formatBytesToMB(value) {
	if (value === null || value === undefined || Number.isNaN(value)) return "--";
	return (Number(value) / 1024 / 1024).toLocaleString(undefined, { maximumFractionDigits: 2 }) + " MB";
}
function parseMetrics(text) {
	const regex = /^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{([^}]*)\})?\s+([0-9.eE+-]+)$/;
	const metrics = {};
	text.split("\n").forEach(line => {
		const trimmed = line.trim();
		if (!trimmed || trimmed.startsWith("#")) return;
		const match = trimmed.match(regex);
		if (!match) return;
		const entry = { labels: {}, value: parseFloat(match[4]) };
		if (match[3]) {
			match[3].split(",").forEach(pair => {
				const idx = pair.indexOf("=");
				if (idx === -1) return;
				entry.labels[pair.slice(0, idx).trim()] = pair.slice(idx + 1).trim().replace(/^"|"$/g, "");
			});
		}
		if (!metrics[match[1]]) metrics[match[1]] = [];
		metrics[match[1]].push(entry);
	});
	return metrics;
}
function getMetric(metrics, name, labels) {
	const list = metrics[name] || [];
	if (!labels) return list.length ? list[0].value : null;
	for (const item of list) {
		let ok = true;
		for (const k in labels) {
			if (item.labels[k] !== labels[k]) { ok = false; break; }
		}
		if (ok) return item.value;
	}
	return null;
}
function sumMetric(metrics, name, filter) {
	return (metrics[name] || []).filter(it => filter ? filter(it.labels) : true).reduce((acc, cur) => acc + (Number.isFinite(cur.value) ? cur.value : 0), 0);
}
function setText(id, value) { document.getElementById(id).textContent = value; }
function renderOverview(m) {
	setText("m_clients_connected", formatNumber(getMetric(m, "gmqtt_clients_connected_total")));
	setText("m_clients_disconnected", formatNumber(getMetric(m, "gmqtt_clients_disconnected_total")));
	setText("m_sessions_active", formatNumber(getMetric(m, "gmqtt_sessions_active_current")));
	setText("m_sessions_inactive", formatNumber(getMetric(m, "gmqtt_sessions_inactive_current")));
	setText("m_sub_current", formatNumber(getMetric(m, "gmqtt_subscriptions_current")));
	setText("m_sub_total", formatNumber(getMetric(m, "gmqtt_subscriptions_total")));
	setText("m_msg_queued", formatNumber(getMetric(m, "gmqtt_messages_queued_current")));
	setText("m_msg_dropped", formatNumber(sumMetric(m, "gmqtt_messages_dropped_total")));
	const goInfo = (m["go_info"] || [])[0];
	setText("m_go_version", goInfo ? (goInfo.labels.version || "--") : "--");
	setText("m_go_goroutines", formatNumber(getMetric(m, "go_goroutines")));
	setText("m_go_threads", formatNumber(getMetric(m, "go_threads")));
	setText("m_proc_cpu", formatNumber(getMetric(m, "process_cpu_seconds_total"), 3));
	setText("m_proc_rss", formatBytesToMB(getMetric(m, "process_resident_memory_bytes")));
	setText("m_proc_fds", formatNumber(getMetric(m, "process_open_fds")));
	setText("m_ph_inflight", formatNumber(getMetric(m, "promhttp_metric_handler_requests_in_flight")));
	setText("m_ph_200", formatNumber(getMetric(m, "promhttp_metric_handler_requests_total", { code: "200" })));
	setText("m_ph_500", formatNumber(getMetric(m, "promhttp_metric_handler_requests_total", { code: "500" })));
	setText("m_ph_503", formatNumber(getMetric(m, "promhttp_metric_handler_requests_total", { code: "503" })));
}
function renderQos(m) {
	document.getElementById("qos-body").innerHTML = ["0", "1", "2"].map(qos => {
		const recv = getMetric(m, "gmqtt_messages_received_total", { qos });
		const sent = getMetric(m, "gmqtt_messages_sent_total", { qos });
		const drop = sumMetric(m, "gmqtt_messages_dropped_total", lab => lab.qos === qos);
		return "<tr><td>QoS " + qos + "</td><td>" + formatNumber(recv) + "</td><td>" + formatNumber(sent) + "</td><td>" + formatNumber(drop) + "</td></tr>";
	}).join("");
}
function renderPackets(m) {
	const names = ["gmqtt_packets_received_total", "gmqtt_packets_received_bytes_total", "gmqtt_packets_sent_total", "gmqtt_packets_sent_bytes_total"];
	const typeSet = new Set();
	names.forEach(name => (m[name] || []).forEach(it => typeSet.add(it.labels.type)));
	document.getElementById("pkt-body").innerHTML = Array.from(typeSet).sort().map(type => {
		return "<tr><td>" + type + "</td><td>" + formatNumber(getMetric(m, names[0], { type })) + "</td><td>" + formatNumber(getMetric(m, names[1], { type })) + "</td><td>" + formatNumber(getMetric(m, names[2], { type })) + "</td><td>" + formatNumber(getMetric(m, names[3], { type })) + "</td></tr>";
	}).join("");
}
async function load() {
	statusEl.className = "status loading";
	statusEl.textContent = "Loading latest metrics...";
	try {
		const res = await fetch(metricsPath, { cache: "no-store" });
		if (!res.ok) throw new Error("HTTP " + res.status);
		const text = await res.text();
		document.getElementById("raw-output").textContent = text;
		const metrics = parseMetrics(text);
		renderOverview(metrics);
		renderQos(metrics);
		renderPackets(metrics);
		statusEl.className = "status success";
		statusEl.textContent = "Loaded at " + new Date().toLocaleTimeString();
	} catch (err) {
		statusEl.className = "status error";
		statusEl.textContent = "Load failed: " + err.message;
	}
}
load();
setInterval(load, 5000);
</script>
</body>
</html>`

func (p *Prometheus) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(dashboardPageTemplate, "__METRICS_PATH__", p.path)
	_, _ = w.Write([]byte(page))
}
