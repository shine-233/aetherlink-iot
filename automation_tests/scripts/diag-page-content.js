// 页面内容验收：判断"页面能用"而不是"页面存在"。
//
// 与 diag-spa-mount.js 的分工：
//   diag-spa-mount.js  —— 能不能挂载（抓 pageerror / 请求失败 / #app 是否为空）
//   diag-page-content.js —— 挂起来之后**主内容区到底有没有东西**
//
// 为什么需要这一层：SPA 普遍有全局外壳（侧边栏、顶栏、面包屑），
// 只断言 document.title 或 body.innerText 会被外壳文本"填满"，
// 把空壳页误判成正常页。本脚本刻意排除外壳，只看主内容区。
//
// 跑法：
//   cd automation_tests && node scripts/diag-page-content.js /a /b /c
//   DIAG_ROLE=super_admin node scripts/diag-page-content.js /apply/service
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const EMPTY_MARKERS = [
  'no data', 'no-data', 'no results', 'no records',
  '暂无数据', '无数据', '暂无记录', '空空如也',
  '敬请期待', 'coming soon', 'under construction', '建设中'
];

(async () => {
  const role = (process.env.DIAG_ROLE || 'tenant_admin').replace(/_/g, '-');
  const storage = path.resolve(__dirname, '..', 'e2e', '.auth', `${role}.json`);
  if (!fs.existsSync(storage)) {
    console.error('storage state missing:', storage);
    process.exit(2);
  }

  const routes = process.argv.slice(2).filter(Boolean);
  if (!routes.length) {
    console.error('usage: node scripts/diag-page-content.js /route [/route ...]');
    process.exit(2);
  }

  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({ storageState: storage, baseURL: 'http://127.0.0.1:9725' });
  const page = await context.newPage();

  const rows = [];

  for (const route of routes) {
    try {
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 15000 });
      await page.waitForTimeout(2500);
    } catch (e) {
      rows.push({ route, verdict: 'GOTO-ERROR', note: e.message.slice(0, 100) });
      continue;
    }

    const info = await page.evaluate(() => {
      // 主内容区候选：优先语义标签，其次常见类名，最后退化为整个 #app。
      const candidates = [
        'main',
        '.n-layout-content',
        '.n-scrollbar-content',
        '#app > div > div:last-child'
      ];
      let main = null;
      for (const sel of candidates) {
        const el = document.querySelector(sel);
        if (el && (el.innerText || '').trim().length > 0) {
          main = el;
          break;
        }
      }
      if (!main) main = document.getElementById('app');

      const mainText = (main && main.innerText || '').trim();
      const title = document.title;

      const countVisible = sel =>
        Array.from(document.querySelectorAll(sel)).filter(el => {
          const r = el.getBoundingClientRect();
          return r.width > 0 && r.height > 0 && getComputedStyle(el).visibility !== 'hidden';
        }).length;

      return {
        title,
        mainText,
        mainLen: mainText.length,
        buttons: countVisible('button'),
        inputs: countVisible('input, textarea'),
        tableRows: countVisible('tbody tr'),
        cards: countVisible('.n-card'),
        is403: /No Permission/i.test(title)
      };
    });

    // 判定必须区分两种"空"：
    //   EMPTY-OK   —— 组件设计的空态：列表没数据，但新增/搜索/分页等交互都在。
    //                 典型文本 "No assets yet, create a root asset first"、
    //                 列表头的 "No Data"。空租户打开列表页本来就该是空的。
    //   EMPTY-BAD  —— 真的没内容：几乎没有控件，或只有一句占位文案。
    // 不看这一点会把"功能完好只是没数据"的列表页误判成坏页，进而错误地不上线。
    const affordances = info.buttons + info.inputs;
    const hasEmptyMarker = EMPTY_MARKERS.some(m => info.mainText.toLowerCase().includes(m));

    let verdict;
    if (info.is403) verdict = '403';
    else if (info.mainLen < 40 && affordances < 3) verdict = 'EMPTY-SHELL';
    else if (hasEmptyMarker && affordances >= 3) verdict = 'EMPTY-OK';
    else if (hasEmptyMarker) verdict = 'EMPTY-BAD';
    else if (affordances <= 1 && info.cards === 0) verdict = 'THIN';
    else verdict = 'OK';

    rows.push({
      route,
      verdict,
      title: info.title,
      mainLen: info.mainLen,
      ui: `btn=${info.buttons} input=${info.inputs} row=${info.tableRows} card=${info.cards}`,
      sample: info.mainText.slice(0, 90).replace(/\s+/g, ' ')
    });
  }

  console.log(`\n角色=${role}  页面内容验收（主内容区，已排除侧边栏/顶栏）\n`);
  for (const r of rows) {
    console.log(`${r.verdict.padEnd(12)} ${r.route}`);
    if (r.title) console.log(`             title="${r.title}" mainLen=${r.mainLen} ${r.ui || ''}`);
    if (r.sample) console.log(`             ${r.sample}`);
    if (r.note) console.log(`             ${r.note}`);
  }

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
