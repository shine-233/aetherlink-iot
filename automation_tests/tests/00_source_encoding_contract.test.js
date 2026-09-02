/**
 * 源码文本编码契约（source encoding contract）。
 *
 * 背景：仓库曾多次出现 UTF-8 源码被按 GBK 保存/转换造成的乱码
 * （注释、E2E 正则、用户可见文案均中过招，见 2026-08 编码修复批次）。
 * 本契约做两件事：
 *   1) 硬性禁止任何被跟踪源文件中出现 U+FFFD 替换符——它代表字节已永久丢失；
 *   2) 用"UTF8-as-GBK 特征字符"启发式拦截典型乱码回潮。特征串取自本仓库
 *      真实事故样本（如 文件用途→鏂囦欢鐢ㄩ€、名称→鍚嶇О、组件配置→缁勪欢閰嶇疆），
 *      正常中文不会包含这些序列；命中即视为编码损坏，必须修复后重提。
 *
 * 局限说明：无 GBK 码表依赖（Node 原生不支持），启发式不能穷尽所有乱码形态；
 * 它是防回归绊线，不是完备证明。新增乱码样本时应把特征串补充进 MOJIBAKE_SIGNATURES。
 */
const fs = require('fs');
const path = require('path');
const { expect } = require('chai');

const projectRoot = path.resolve(__dirname, '..', '..');

const SCAN_DIRS = [
  'backend',
  'mqtt-broker',
  'frontend/src',
  'automation_tests/lib',
  'automation_tests/tests',
  'automation_tests/e2e',
  'automation_tests/scripts',
  'automation_tests/coverage-contract',
  'deploy',
  'references'
];

const SCAN_EXTENSIONS = new Set([
  '.go', '.vue', '.ts', '.js', '.mjs', '.cjs', '.sql', '.yml', '.yaml',
  '.json', '.md', '.sh', '.ps1', '.cmd', '.html', '.css'
]);

const SKIP_PATTERN = /node_modules|(^|\/)dist(\/|$)|testdata|__snapshots__|(^|\/)output(\/|$)|\.min\./;

// 取自本仓库真实乱码事故的特征序列（均为"UTF-8 字节被按 GBK 重解释"的产物，
// 正常简体中文技术文本不会出现这些连续组合）。
const MOJIBAKE_SIGNATURES = [
  '鏂囦欢', '鐢ㄩ€', '鍚嶇О', '璁块棶', '璺緞', '缂栬緫', '妯″紡',
  '缁勪欢', '浜や簰', '鏍稿績', '閫昏緫', '鍏抽键', '鍏抽敭', '娴嬭瘯',
  '鐘舵€', '鏁版嵁', '鑾峰彇', '鍔犺浇', '璁惧', '閰嶇疆', '鍛戒护',
  '鍘嗗彶', '鍒嗕韩', '娓╁害', '鍛婅', '妫€娴', '瑙嗗浘', '缂佸嫪',
  '鎵句笉鍒?', '鏈嵁', '璇锋眰', '鎼滅储', '鍒楄〃', '琛ㄥ崟', '寮圭獥'
];

function* walk(dir) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (err) {
    return;
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    const relative = path.relative(projectRoot, full).replace(/\\/g, '/');
    if (entry.isDirectory()) {
      if (!SKIP_PATTERN.test(relative + '/')) {
        yield* walk(full);
      }
    } else if (entry.isFile()) {
      if (!SKIP_PATTERN.test(relative) && SCAN_EXTENSIONS.has(path.extname(entry.name))) {
        yield { full, relative };
      }
    }
  }
}

// 契约测试自身包含乱码特征样本表，必须排除在扫描范围外。
const SELF_PATH = 'automation_tests/tests/00_source_encoding_contract.test.js';

function inspectSourceEncoding() {
  const findings = [];
  for (const dir of SCAN_DIRS) {
    const absolute = path.join(projectRoot, dir);
    if (!fs.existsSync(absolute)) {
      continue;
    }
    for (const file of walk(absolute)) {
      if (file.relative === SELF_PATH) {
        continue;
      }
    let content;
    try {
      content = fs.readFileSync(file.full, 'utf8');
    } catch (err) {
      findings.push({ file: file.relative, line: 0, reason: 'not-valid-utf8' });
      continue;
    }
    // 超大文件不纳入契约范围（当前仓库不存在此类源文件）；读取后按字节长度判断，避免 stat+read 文件竞态。
    if (Buffer.byteLength(content, 'utf8') > 4 * 1024 * 1024) {
      continue;
    }
    const lines = content.split('\n');
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (line.includes('\uFFFD')) {
        findings.push({ file: file.relative, line: i + 1, reason: 'U+FFFD replacement char (bytes already lost)' });
        break;
      }
      const hit = MOJIBAKE_SIGNATURES.find(sig => line.includes(sig));
      if (hit) {
        findings.push({ file: file.relative, line: i + 1, reason: `mojibake signature "${hit}"` });
        break;
      }
    }
    }
  }
  return findings;
}

describe('source encoding contract [00_source_encoding_contract]', function () {
  this.timeout(60000);

  it('keeps every scanned source file free of U+FFFD and known mojibake signatures', function () {
    const findings = inspectSourceEncoding();
    expect(
      findings,
      `encoding damage detected (fix the files, do not skip this contract):\n${findings
        .map(f => `  ${f.file}:${f.line} ${f.reason}`)
        .join('\n')}`
    ).to.deep.equal([]);
  });

  it('flags a synthetic mojibake line through the same detector used for the repository scan', function () {
    // 自证：检测器对已知坏样本必须报警，防止签名表被清空后契约静默失效。
    const damaged = '// 鏂囦欢鐢ㄩ€? RDI 璁惧璇︽儏';
    const hit = MOJIBAKE_SIGNATURES.find(sig => damaged.includes(sig));
    expect(hit, 'signature table must catch the archived incident sample').to.be.a('string');
  });
});
