// 诊断脚本：捕获 SPA 启动期的 console / pageerror / 失败请求，以及 #app 内部 HTML。
// 跑法：cd automation_tests && node scripts/diag-spa-mount.js
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

(async () => {
  const storage = path.resolve(__dirname, '..', 'e2e', '.auth', 'tenant-admin.json');
  if (!fs.existsSync(storage)) {
    console.error('storage state missing:', storage);
    process.exit(2);
  }

  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({ storageState: storage, baseURL: 'http://127.0.0.1:9725' });
  const page = await context.newPage();

  const logs = [];
  page.on('console', msg => logs.push(`[console.${msg.type()}] ${msg.text()}`));
  page.on('pageerror', err => logs.push(`[pageerror] ${err.name}: ${err.message}\n${err.stack || ''}`));
  page.on('requestfailed', req => logs.push(`[requestfailed] ${req.method()} ${req.url()} -> ${req.failure() && req.failure().errorText}`));
  page.on('response', resp => {
    if (resp.status() >= 400) logs.push(`[response.${resp.status()}] ${resp.request().method()} ${resp.url()}`);
  });

  const ROUTES = ['/login', '/visualization/anomaly', '/visualization/report', '/market/browse', '/management/edge-nodes', '/management/license', '/device/grouping'];

  for (const route of ROUTES) {
    logs.push(`\n========== goto ${route} ==========`);
    try {
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 15000 });
      await page.waitForTimeout(3000);
    } catch (e) {
      logs.push(`[goto.error] ${e.message}`);
    }
    const info = await page.evaluate(() => {
      const app = document.getElementById('app');
      return {
        url: location.href,
        title: document.title,
        appChildren: app ? app.children.length : -1,
        appHTMLLen: app ? app.innerHTML.length : -1,
        appTextSample: app ? (app.innerText || '').slice(0, 200) : '',
        bodyTextSample: (document.body.innerText || '').slice(0, 200),
        hasRoot: !!document.querySelector('#app > *')
      };
    });
    logs.push(`[info] url=${info.url} title="${info.title}" appChildren=${info.appChildren} appHTMLLen=${info.appHTMLLen} hasRoot=${info.hasRoot}`);
    logs.push(`[body.text] ${info.bodyTextSample.replace(/\n/g, ' | ')}`);
  }

  console.log(logs.join('\n'));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });