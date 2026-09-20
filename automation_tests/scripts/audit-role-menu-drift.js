// 角色 × 路由 一致性审计：抓"菜单放行了，但接口不放行"的漂移。
//
// ## 为什么需要这个
//
// 本项目的授权是**两套机制配合**的：
//   1. 后端 Casbin 决定某个角色的 API 能不能调（失败即 403，fail-closed）；
//   2. 前端菜单（sys_ui_elements.authority）决定某个角色看不看得到入口。
//
// 官方设计就是这么说的（sql/64.sql 文件头）：
//   "收紧后 TENANT_USER/TENANT_ADMIN 调用被撤端点将收到 403（RBAC fail-closed），
//    这是预期行为；console 对应页面按角色菜单过滤（/ui_elements/menu）不展示入口。"
//
// 也就是说：**菜单必须和后端撤权保持一致**。一旦不一致，就产生一类很隐蔽的故障：
// 页面能打开（菜单放行），但页面自己的数据接口 403（后端拒绝）——
// 用户看到的是一个永远加载不出数据的空壳页，还伴随 pageerror。
//
// 2026-09-15 实际发生过一次：sql/47.sql 用一条"按 authority 内容匹配"的批量 UPDATE
// 给所有含 TENANT_ADMIN 的菜单行统一追加了 TENANT_USER，把 64.sql 的撤权打穿，
// 导致 /management/api 对 tenant_user 变成空壳页。
//
// ## 设计要点：不维护"路由 → 接口"映射
//
// 那种映射既不存在也没人维护，写死必然腐坏。本脚本改成让浏览器自己回答：
//   - 页面落到 403  → 菜单没放行 → 正常，跳过
//   - 页面渲染出来，但该页发起的 API 请求有 4xx/5xx → **DRIFT**
//   - 页面渲染出来且 API 全通 → OK
//
// ## 跑法
//
//   cd automation_tests
//   node scripts/audit-role-menu-drift.js /a /b /c              # 默认扫 tenant_admin + tenant_user
//   AUDIT_ROLES=tenant_user,readonly_user node scripts/audit-role-menu-drift.js /a /b
//
// 有 DRIFT 时以退出码 1 结束，可直接接进 CI。

const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const AUTH_DIR = path.resolve(__dirname, '..', 'e2e', '.auth');

/** 角色 → 登录态文件名（与 e2e/fixtures.js 的 ROLE_FILES 保持一致） */
const ROLE_FILES = {
  super_admin: 'super-admin.json',
  tenant_admin: 'tenant-admin.json',
  tenant_user: 'tenant-user.json',
  tenant_admin_b: 'tenant-admin-b.json',
  readonly_user: 'readonly-user.json',
  email_change_tenant: 'email-change-tenant.json'
};

/** 只关心业务接口的失败，静态资源与遥测埋点不算 */
function isApiUrl(url) {
  return /\/api\/v\d+\//.test(url);
}

function storageStateFor(role) {
  const file = ROLE_FILES[role];
  if (!file) return null;
  const full = path.join(AUTH_DIR, file);
  return fs.existsSync(full) ? full : null;
}

async function auditRole(browser, baseURL, role, routes) {
  const storageState = storageStateFor(role);
  if (!storageState) {
    return { role, skipped: `登录态文件不存在（${ROLE_FILES[role] || '未登记的角色'}）`, rows: [] };
  }

  const context = await browser.newContext({ storageState, baseURL });
  const page = await context.newPage();

  const rows = [];

  for (const route of routes) {
    const apiFailures = [];

    const onResponse = resp => {
      if (resp.status() >= 400 && isApiUrl(resp.url())) {
        apiFailures.push(`${resp.status()} ${resp.request().method()} ${resp.url().replace(baseURL, '')}`);
      }
    };
    page.on('response', onResponse);

    let blocked = false;
    try {
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 20000 });
      await page.waitForTimeout(2200);
      blocked = await page.evaluate(() => /No Permission/i.test(document.title));
    } catch (e) {
      page.off('response', onResponse);
      rows.push({ route, verdict: 'GOTO-ERROR', detail: e.message.slice(0, 80) });
      continue;
    }

    page.off('response', onResponse);

    if (blocked) {
      // 菜单没放行 —— 这是正确行为，不是漂移
      rows.push({ route, verdict: 'blocked' });
    } else if (apiFailures.length > 0) {
      // 菜单放行了，但接口拒绝 —— 这就是漂移
      rows.push({ route, verdict: 'DRIFT', detail: [...new Set(apiFailures)].slice(0, 3) });
    } else {
      rows.push({ route, verdict: 'ok' });
    }
  }

  await context.close();
  return { role, rows };
}

(async () => {
  const baseURL = process.env.AUDIT_BASE_URL || 'http://127.0.0.1:9725';
  const routes = process.argv.slice(2).filter(Boolean);
  if (!routes.length) {
    console.error('usage: node scripts/audit-role-menu-drift.js /route [/route ...]');
    console.error('   env: AUDIT_ROLES=tenant_admin,tenant_user  AUDIT_BASE_URL=http://127.0.0.1:9725');
    process.exit(2);
  }

  const roles = (process.env.AUDIT_ROLES || 'tenant_admin,tenant_user')
    .split(',')
    .map(s => s.trim())
    .filter(Boolean);

  const browser = await chromium.launch({ channel: 'msedge', headless: true });

  let driftCount = 0;
  const summary = [];

  for (const role of roles) {
    const { skipped, rows } = await auditRole(browser, baseURL, role, routes);

    console.log(`\n========== 角色 ${role} ==========`);
    if (skipped) {
      console.log(`  跳过：${skipped}`);
      continue;
    }

    for (const row of rows) {
      if (row.verdict === 'DRIFT') {
        driftCount++;
        console.log(`  DRIFT  ${row.route}`);
        row.detail.forEach(d => console.log(`         接口失败：${d}`));
      } else if (row.verdict === 'GOTO-ERROR') {
        console.log(`  ERROR  ${row.route}  ${row.detail}`);
      } else if (row.verdict === 'blocked') {
        console.log(`  -      ${row.route}  （菜单未放行，正常）`);
      } else {
        console.log(`  ok     ${row.route}`);
      }
    }

    const reachable = rows.filter(r => r.verdict === 'ok' || r.verdict === 'DRIFT').length;
    summary.push(`${role}: 可达 ${reachable} 条，漂移 ${rows.filter(r => r.verdict === 'DRIFT').length} 条`);
  }

  console.log('\n========== SUMMARY ==========');
  summary.forEach(s => console.log('  ' + s));
  console.log(`\n共发现 ${driftCount} 处漂移（菜单放行但接口拒绝）。`);

  await browser.close();
  process.exit(driftCount > 0 ? 1 : 0);
})().catch(e => {
  console.error('FATAL', e);
  process.exit(2);
});
