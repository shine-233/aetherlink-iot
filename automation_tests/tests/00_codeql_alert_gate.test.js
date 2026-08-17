/**
 * Prevents the CodeQL alert gate from passing on an unanalysed or mismatched
 * pull-request ref.  The hosted job is the executable check; this contract
 * keeps its ref, commit, tool, and fail-closed requirements reviewable in CI.
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..');

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), 'utf8');
}

describe('CodeQL alert gate contract [00_codeql_alert_gate]', function () {
  const source = read('.github/workflows/codeql.yml');

  it('checks the ref and commit that the workflow actually analysed', function () {
    expect(source).to.match(/CODEQL_REF:\s+\$\{\{\s*github\.ref\s*\}\}/);
    expect(source).to.match(/CODEQL_COMMIT:\s+\$\{\{\s*github\.sha\s*\}\}/);
    expect(source).to.include('code-scanning/analyses');
    expect(source).to.include('-f tool_name=CodeQL');
  });

  it('fails closed when any successful language analysis is missing', function () {
    expect(source).to.include('required_categories=(');
    expect(source).to.include('missing_categories=');
    expect(source).to.include('no successful CodeQL analysis');
    expect(source).to.match(/missing_categories[\s\S]*?exit 1/);
  });
});
