const Mocha = require('mocha');

const caseContext = require('./coverage_case_context');
const delegate = require('mochawesome');

const {
  EVENT_TEST_BEGIN,
  EVENT_TEST_END,
  EVENT_HOOK_BEGIN,
  EVENT_HOOK_END
} = Mocha.Runner.constants;

module.exports = function CoverageMochawesomeReporter(runner, options) {
  let active = null;

  const buildTestContext = test => ({
    runId: process.env.AETHERLINK_COVERAGE_RUN_ID,
    module: process.env.AETHERLINK_COVERAGE_MODULE,
    case: {
      file: test.file || null,
      title: test.title || null,
      titlePath: typeof test.fullTitle === 'function' ? [test.fullTitle()] : [],
      caseId: null
    },
    attempt: { retry: typeof test.currentRetry === 'function' ? test.currentRetry() : 0 }
  });

  runner.on(EVENT_TEST_BEGIN, test => {
    active = buildTestContext(test);
    caseContext.run(active, () => {});
  });

  runner.on(EVENT_HOOK_BEGIN, hook => {
    active = {
      runId: process.env.AETHERLINK_COVERAGE_RUN_ID,
      module: process.env.AETHERLINK_COVERAGE_MODULE,
      case: {
        file: hook.file || null,
        hook: hook.title || hook.hookName || 'hook'
      },
      attempt: {}
    };
    caseContext.run(active, () => {});
  });

  runner.on(EVENT_TEST_END, () => { active = null; });
  runner.on(EVENT_HOOK_END, () => { active = null; });

  const originalRun = runner.runTest.bind(runner);
  runner.runTest = function(done) {
    const context = this.test ? buildTestContext(this.test) : active;
    return caseContext.run(context, () => originalRun(done));
  };

  const originalHook = runner.hook.bind(runner);
  runner.hook = function(name, fn) {
    const context = active;
    return caseContext.run(context, () => originalHook(name, fn));
  };

  delegate.call(this, runner, options);
};
