/**
 * 文件用途：视觉回归基座采集脚本：按页面集×双视口截图生成前后对比锚点，支持新旧目录粗对比。
 * 核心逻辑：复用 e2e/.auth/super-admin.json 登录态（与 playwright.config.js globalSetup 产物同源），
 *           缺失或 origin 不匹配时用 SUPER_ADMIN_EMAIL/PASSWORD 走 API 登录并按 e2e specs 的写法注入
 *           localStorage（token/token_expires_in/userInfo）；逐页等待 networkidle+500ms 后截图到
 *           visual-baseline/<timestamp>/ 并写 manifest.json；--compare 对两目录同名 PNG 做尺寸+字节粗判。
 * 关键注意事项：本脚本不进入 playwright runner 自动发现，仅手动/workflow 按需调用；输出目录已被根
 *           .gitignore 忽略；字节相等是保守判定，动态时间戳/字体渲染差异会误报；像素级 diff 不引入
 *           新依赖，后续可换 pixelmatch。
 */

const fs = require('fs');
const path = require('path');
const axios = require('axios');
const { chromium } = require('@playwright/test');
const config = require('../lib/network_runtime');

const BASE_URL = process.env.FRONTEND_URL || 'http://127.0.0.1:9725';
const TARGET_ORIGIN = new URL(BASE_URL).origin;
const OUTPUT_ROOT = path.resolve(__dirname, '..', 'visual-baseline');
const AUTH_DIR = path.resolve(__dirname, '..', config.e2e.storageStateDir);
const SUPER_ADMIN_STATE_FILE = path.join(AUTH_DIR, 'super-admin.json');

const VIEWPORTS = [
  { name: 'desktop', width: 1280, height: 800 },
  { name: 'mobile-390x844', width: 390, height: 844 }
];

function buildPageSet() {
  const pages = [
    { name: 'login', urlPath: '/login', anonymous: true },
    { name: 'dashboard', urlPath: '/dashboard' },
    { name: 'alarm-warning-message', urlPath: '/alarm/warning-message' },
    { name: 'device-manage', urlPath: '/device/manage' }
  ];
  const deviceId = process.env.DEVICE_ID || '';
  if (deviceId) {
    pages.push({
      name: 'device-details',
      urlPath: `/device/details?d_id=${encodeURIComponent(deviceId)}`
    });
  } else {
    console.log('[pages] DEVICE_ID not set, skipping /device/details');
  }
  return pages;
}

async function apiLoginSuperAdmin() {
  const email = process.env.SUPER_ADMIN_EMAIL || '';
  const password = process.env.SUPER_ADMIN_PASSWORD || '';
  if (!email || !password) {
    return null;
  }

  const client = axios.create({
    baseURL: config.baseURL,
    timeout: config.timeout,
    headers: { 'Content-Type': 'application/json' }
  });

  const loginResp = await client.post('/login', { email, password, salt: null });
  const loginBody = loginResp.data;
  const loginToken = loginBody && loginBody.code === 200 ? loginBody.data : null;
  if (!loginToken || !loginToken.token) {
    throw new Error(`super_admin API login returned no token: ${JSON.stringify(loginBody)}`);
  }

  const detailResp = await client.get('/user/detail', {
    headers: { 'x-token': loginToken.token }
  });
  const detailBody = detailResp.data;
  const userInfo = detailBody && detailBody.code === 200 ? detailBody.data : null;
  if (!userInfo) {
    throw new Error(`user/detail returned no data: ${JSON.stringify(detailBody)}`);
  }

  const expiresIn = Number(loginToken.expires_in || loginToken.expiresIn || 7200);
  const normalizedUserInfo = {
    ...userInfo,
    roles: Array.isArray(userInfo.roles) && userInfo.roles.length
      ? userInfo.roles
      : [userInfo.authority].filter(Boolean)
  };

  return {
    cookies: [],
    origins: [
      {
        origin: TARGET_ORIGIN,
        localStorage: [
          { name: 'token', value: JSON.stringify(loginToken.token) },
          { name: 'token_expires_in', value: JSON.stringify(String(Date.now() + expiresIn * 1000)) },
          { name: 'userInfo', value: JSON.stringify(normalizedUserInfo) }
        ]
      }
    ]
  };
}

async function resolveAuthState() {
  if (fs.existsSync(SUPER_ADMIN_STATE_FILE)) {
    try {
      const state = JSON.parse(fs.readFileSync(SUPER_ADMIN_STATE_FILE, 'utf8'));
      const origins = (state.origins || []).map(item => item.origin);
      if (origins.includes(TARGET_ORIGIN)) {
        return { state, mode: 'storage-state-file' };
      }
      console.warn(
        `[auth] ${SUPER_ADMIN_STATE_FILE} origins ${JSON.stringify(origins)} do not cover ${TARGET_ORIGIN}; falling back to API login`
      );
    } catch (error) {
      console.warn(`[auth] failed to read ${SUPER_ADMIN_STATE_FILE}: ${error.message}`);
    }
  }

  const state = await apiLoginSuperAdmin();
  if (state) {
    return { state, mode: 'api-login' };
  }
  console.warn(
    `[auth] no reusable super-admin storageState at ${SUPER_ADMIN_STATE_FILE} and no SUPER_ADMIN_EMAIL/SUPER_ADMIN_PASSWORD; authenticated pages will be skipped`
  );
  return { state: null, mode: 'anonymous-only' };
}

async function capture() {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const outDir = path.join(OUTPUT_ROOT, timestamp);
  fs.mkdirSync(outDir, { recursive: true });

  const pages = buildPageSet();
  const auth = await resolveAuthState();
  const executablePath = process.env.PLAYWRIGHT_BROWSER_EXECUTABLE_PATH;
  const launchOptions = executablePath
    ? { executablePath, headless: true }
    : { channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || 'msedge', headless: true };

  const browser = await chromium.launch(launchOptions);
  const manifest = {
    generatedAt: new Date().toISOString(),
    baseURL: BASE_URL,
    apiBaseURL: config.baseURL,
    authMode: auth.mode,
    deviceId: process.env.DEVICE_ID || null,
    viewports: VIEWPORTS,
    captures: []
  };
  let failures = 0;

  try {
    for (const entry of pages) {
      const needsAuth = !entry.anonymous;
      if (needsAuth && !auth.state) {
        for (const viewport of VIEWPORTS) {
          manifest.captures.push({
            url: new URL(entry.urlPath, BASE_URL).toString(),
            page: entry.name,
            viewport: { width: viewport.width, height: viewport.height },
            filename: `${entry.name}_${viewport.width}x${viewport.height}.png`,
            authenticated: true,
            status: 'skipped',
            reason: 'no-auth-state'
          });
          failures++;
        }
        continue;
      }

      for (const viewport of VIEWPORTS) {
        const context = await browser.newContext({
          viewport: { width: viewport.width, height: viewport.height },
          ...(needsAuth ? { storageState: auth.state } : {})
        });
        const page = await context.newPage();
        const targetURL = new URL(entry.urlPath, BASE_URL).toString();
        const filename = `${entry.name}_${viewport.width}x${viewport.height}.png`;
        const record = {
          url: targetURL,
          page: entry.name,
          viewport: { width: viewport.width, height: viewport.height },
          filename,
          authenticated: needsAuth,
          status: 'ok'
        };

        try {
          await page.goto(targetURL, { waitUntil: 'networkidle', timeout: 30000 });
          await page.waitForTimeout(500);
          await page.screenshot({ path: path.join(outDir, filename), fullPage: false });
          record.takenAt = new Date().toISOString();
          console.log(`[shot] ${filename} <- ${targetURL}`);
        } catch (error) {
          record.status = 'error';
          record.error = error.message;
          failures++;
          console.error(`[shot] FAILED ${filename} <- ${targetURL}: ${error.message}`);
        } finally {
          await context.close();
          manifest.captures.push(record);
        }
      }
    }
  } finally {
    await browser.close();
  }

  fs.writeFileSync(path.join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2), 'utf8');
  console.log(`[done] ${manifest.captures.filter(c => c.status === 'ok').length} screenshots -> ${outDir}`);
  if (failures > 0) {
    console.error(`[done] ${failures} capture(s) skipped or failed`);
    process.exitCode = 1;
  }
}

function listPNGs(dir) {
  if (!fs.existsSync(dir)) {
    throw new Error(`directory not found: ${dir}`);
  }
  return fs.readdirSync(dir).filter(name => name.toLowerCase().endsWith('.png')).sort();
}

function latestBaselineDir(excludeResolved) {
  const candidates = fs.readdirSync(OUTPUT_ROOT, { withFileTypes: true })
    .filter(entry => entry.isDirectory())
    .map(entry => path.join(OUTPUT_ROOT, entry.name))
    .filter(dir => path.resolve(dir) !== excludeResolved)
    .sort()
    .reverse();
  return candidates[0] || null;
}

function compare(oldDirRaw, newDirRaw) {
  const oldDir = path.resolve(oldDirRaw);
  const newDir = newDirRaw
    ? path.resolve(newDirRaw)
    : latestBaselineDir(oldDir);

  if (!newDir) {
    console.error('[compare] no second baseline directory found under visual-baseline/; pass it explicitly:');
    console.error('         npm run visual:compare -- <oldDir> <newDir>');
    process.exitCode = 1;
    return;
  }

  const oldFiles = new Set(listPNGs(oldDir));
  const newFiles = new Set(listPNGs(newDir));
  const common = [...oldFiles].filter(name => newFiles.has(name));
  const differences = [];

  for (const name of common) {
    const oldBuffer = fs.readFileSync(path.join(oldDir, name));
    const newBuffer = fs.readFileSync(path.join(newDir, name));
    if (oldBuffer.length !== newBuffer.length || !oldBuffer.equals(newBuffer)) {
      differences.push({
        name,
        oldBytes: oldBuffer.length,
        newBytes: newBuffer.length,
        verdict: oldBuffer.length !== newBuffer.length ? 'size-mismatch' : 'bytes-differ'
      });
    }
  }

  const missingInOld = [...newFiles].filter(name => !oldFiles.has(name));
  const missingInNew = [...oldFiles].filter(name => !newFiles.has(name));

  console.log(`[compare] old=${oldDir}`);
  console.log(`[compare] new=${newDir}`);
  console.log(`[compare] common=${common.length} identical=${common.length - differences.length}`);

  for (const diff of differences) {
    console.log(`DIFF ${diff.name}: ${diff.verdict} (old=${diff.oldBytes}B new=${diff.newBytes}B)`);
  }
  for (const name of missingInOld) {
    console.log(`MISSING-IN-OLD ${name}`);
  }
  for (const name of missingInNew) {
    console.log(`MISSING-IN-NEW ${name}`);
  }

  if (differences.length || missingInOld.length || missingInNew.length) {
    console.error('[compare] baselines differ; see list above (byte-level coarse check; swap in pixelmatch later for pixel diffs)');
    process.exitCode = 1;
  } else {
    console.log('[compare] baselines byte-identical');
  }
}

function parseArgs(argv) {
  const args = { mode: 'capture', positional: [] };
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--compare') {
      args.mode = 'compare';
    } else if (argv[i] === '--help' || argv[i] === '-h') {
      args.help = true;
    } else {
      args.positional.push(argv[i]);
    }
  }
  return args;
}

function printUsage() {
  console.log('Usage: node scripts/capture_visual_baseline.js [--compare <oldDir> [<newDir>]]');
  console.log('');
  console.log('Capture: screenshots of /login, /dashboard, /alarm/warning-message, /device/manage');
  console.log('         (+ /device/details?d_id=$DEVICE_ID when set) at 1280x800 and 390x844 into');
  console.log('         visual-baseline/<timestamp>/ with manifest.json.');
  console.log('Compare: npm run visual:compare -- <oldDir> [<newDir>] ; defaults <newDir> to the');
  console.log('         latest other baseline directory. Exit code 1 on any difference.');
  console.log('Env: FRONTEND_URL (default http://127.0.0.1:9725), API_BASE_URL, DEVICE_ID,');
  console.log('     SUPER_ADMIN_EMAIL/SUPER_ADMIN_PASSWORD, E2E_AUTH_DIR, PLAYWRIGHT_BROWSER_CHANNEL.');
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    printUsage();
    return;
  }
  if (args.mode === 'compare') {
    const oldDir = args.positional[0] || process.env.VISUAL_COMPARE_DIR;
    if (!oldDir) {
      printUsage();
      process.exitCode = 1;
      return;
    }
    compare(oldDir, args.positional[1]);
    return;
  }
  await capture();
}

main().catch(error => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
