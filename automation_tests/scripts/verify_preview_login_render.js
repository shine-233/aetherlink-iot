/**
 * 文件用途：用于执行预览登录页渲染验证脚本。
 * 核心逻辑：作为独立 Node 脚本编排本地预检、账号准备、预览代理或页面渲染验证，并输出可诊断结果。
 * 关键注意事项：运行前必须确认目标环境、账号和端口配置，避免把预检失败误判为业务失败。
 * 重构建议：后续应把环境解析、错误分类和可复用检查步骤抽到共享库，保持脚本入口薄而明确。
 */

const assert = require('assert');
const { chromium } = require('@playwright/test');

const previewURL = process.env.PREVIEW_URL || process.env.FRONTEND_URL || 'http://127.0.0.1:9725';
const browserChannel = process.env.PLAYWRIGHT_BROWSER_CHANNEL || 'msedge';

async function main() {
  const browser = await chromium.launch({ channel: browserChannel, headless: true });
  const page = await browser.newPage();
  const pageErrors = [];
  const failedRequests = [];

  page.on('pageerror', error => {
    if (error && typeof error === 'object') {
      pageErrors.push({
        name: error.name,
        message: error.message,
        stack: error.stack,
        value: String(error)
      });
      return;
    }
    pageErrors.push(String(error));
  });
  page.on('requestfailed', request => {
    failedRequests.push({
      url: request.url(),
      method: request.method(),
      failure: request.failure() && request.failure().errorText
    });
  });

  try {
    await page.goto(new URL('/login', previewURL).toString(), {
      waitUntil: 'domcontentloaded',
      timeout: 30000
    });
    await page.waitForTimeout(3000);

    const state = await page.evaluate(() => ({
      href: window.location.href,
      title: document.title,
      appHtmlLength: document.querySelector('#app')?.innerHTML.length || 0,
      inputCount: document.querySelectorAll('input').length,
      buttonCount: document.querySelectorAll('button').length,
      bodyText: document.body ? document.body.innerText.slice(0, 500) : ''
    }));

    assert.strictEqual(pageErrors.length, 0, 'Preview login has page errors: ' + JSON.stringify(pageErrors, null, 2));
    assert(state.appHtmlLength > 0, 'Preview app did not mount into #app');
    assert(state.inputCount >= 2, 'Preview login should render editable login inputs');
    assert(state.buttonCount >= 1, 'Preview login should render at least one button');

    console.log(
      JSON.stringify(
        {
          previewURL,
          browserChannel,
          state,
          failedRequests
        },
        null,
        2
      )
    );
  } finally {
    await browser.close();
  }
}

main().catch(error => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
