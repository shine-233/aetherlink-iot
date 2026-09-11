#!/usr/bin/env node
const { auditProductionPlaceholders } = require('../lib/production-placeholder-audit');

function main(options = {}) {
  const audit = options.audit || auditProductionPlaceholders;
  const stdout = options.stdout || process.stdout;
  const result = audit(options.auditOptions || {});
  stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  return result.ok ? 0 : 1;
}

if (require.main === module) process.exitCode = main();

module.exports = { main };
