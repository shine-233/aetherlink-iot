/**
 * 文件用途：用于支撑 automation_tests 的集成阻塞原因记录模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const blockedCases = [];

function normalizeReason(reason) {
  if (reason && typeof reason === 'string') {
    return { reason };
  }
  if (reason && typeof reason === 'object') {
    return {
      reason: typeof reason.reason === 'string' ? reason.reason : 'integration prerequisite is unavailable in the current local environment',
      category: typeof reason.category === 'string' ? reason.category : undefined,
      seedable: typeof reason.seedable === 'boolean' ? reason.seedable : undefined
    };
  }
  return { reason: 'integration prerequisite is unavailable in the current local environment' };
}

function recordBlocked(reason) {
  const normalized = normalizeReason(reason);
  blockedCases.push({
    ...normalized,
    at: new Date().toISOString()
  });
  return normalized;
}

function printBlockedReason(normalized) {
  // Keep the readable line for existing Mocha/Playwright logs, but also emit
  // the original metadata so the runner does not have to reclassify it from
  // prose and accidentally turn runtime-external into seedable-local.
  console.warn('  integration-blocked: ' + normalized.reason);
  console.warn('  integration-blocked-meta: ' + JSON.stringify(normalized));
}

function skipIfBlocked(context, reason) {
  const normalized = recordBlocked(reason);
  if (!context || typeof context.skip !== 'function') {
    throw new Error('Cannot mark integration-blocked without a Mocha context: ' + normalized.reason);
  }
  printBlockedReason(normalized);
  context.skip();
}

function skipWhenBlocked(testLike, condition, reason) {
  const normalized = normalizeReason(reason);
  if (condition) {
    recordBlocked(normalized);
    printBlockedReason(normalized);
  }
  testLike.skip(condition, normalized.reason);
}

function getBlockedCases() {
  return blockedCases.slice();
}

function resetBlockedCases() {
  blockedCases.length = 0;
}

module.exports = {
  getBlockedCases,
  resetBlockedCases,
  skipIfBlocked,
  skipWhenBlocked
};
