/**
 * 文件用途：用于支撑 automation_tests 的本地文件系统进程锁模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function acquireProcessLock(name, options = {}) {
  const timeoutMs = options.timeoutMs || 120000;
  const staleMs = options.staleMs || 300000;
  const lockDir = path.join(__dirname, '..', '.locks');
  const lockPath = path.join(lockDir, name + '.lock');
  const start = Date.now();

  fs.mkdirSync(lockDir, { recursive: true });

  while (true) {
    try {
      const fd = fs.openSync(lockPath, 'wx');
      fs.writeFileSync(fd, JSON.stringify({ pid: process.pid, createdAt: new Date().toISOString() }));
      fs.closeSync(fd);

      let released = false;
      return async function releaseProcessLock() {
        if (released) return;
        released = true;
        try {
          fs.unlinkSync(lockPath);
        } catch (error) {
          if (error.code !== 'ENOENT') throw error;
        }
      };
    } catch (error) {
      if (error.code !== 'EEXIST') throw error;

      try {
        const stat = fs.statSync(lockPath);
        if (Date.now() - stat.mtimeMs > staleMs) {
          fs.unlinkSync(lockPath);
          continue;
        }
      } catch (statError) {
        if (statError.code !== 'ENOENT') throw statError;
      }

      if (Date.now() - start > timeoutMs) {
        throw new Error('Timed out waiting for process lock: ' + lockPath);
      }

      await sleep(250);
    }
  }
}

module.exports = acquireProcessLock;
