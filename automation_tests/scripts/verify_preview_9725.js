/**
 * 文件用途：用于执行9725 预览页面验证脚本。
 * 核心逻辑：作为独立 Node 脚本编排本地预检、账号准备、预览代理或页面渲染验证，并输出可诊断结果。
 * 关键注意事项：运行前必须确认目标环境、账号和端口配置，避免把预检失败误判为业务失败。
 * 重构建议：后续应把环境解析、错误分类和可复用检查步骤抽到共享库，保持脚本入口薄而明确。
 */

const assert = require('assert');

const previewURL = process.env.PREVIEW_URL || 'http://127.0.0.1:9725';

async function fetchText(url) {
  const resp = await fetch(url, { signal: AbortSignal.timeout(15000) });
  const text = await resp.text();
  return { resp, text };
}

function toAbsoluteURL(path) {
  return new URL(path, previewURL).toString();
}

function extractLocalAssets(html) {
  const assets = [];
  const pattern = /(?:src|href)="([^"]+)"/g;
  let match;

  while ((match = pattern.exec(html)) !== null) {
    const assetPath = match[1];
    if (assetPath.startsWith('/assets/')) {
      assets.push(assetPath);
    }
  }

  return [...new Set(assets)];
}

async function main() {
  const { resp, text } = await fetchText(previewURL + '/');
  assert.strictEqual(resp.status, 200, `Expected preview root HTTP 200, got ${resp.status}`);
  assert.match(text, /<title>AetherLink IoT<\/title>/, 'Preview root should return the AetherLink IoT document');
  assert.match(text, /<div id="app"><\/div>/, 'Preview root should contain the Vue mount node');

  const assets = extractLocalAssets(text);
  assert(assets.length >= 3, `Expected multiple built assets in preview HTML, got ${assets.length}`);

  const checkedAssets = assets.slice(0, 8);
  for (const asset of checkedAssets) {
    const assetResp = await fetch(toAbsoluteURL(asset), {
      method: 'HEAD',
      signal: AbortSignal.timeout(15000)
    });
    assert.strictEqual(assetResp.status, 200, `Expected asset ${asset} HTTP 200, got ${assetResp.status}`);
  }

  console.log(
    JSON.stringify(
      {
        previewURL,
        rootStatus: resp.status,
        htmlBytes: Buffer.byteLength(text),
        assetsDiscovered: assets.length,
        assetsChecked: checkedAssets.length
      },
      null,
      2
    )
  );
}

main().catch(error => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
