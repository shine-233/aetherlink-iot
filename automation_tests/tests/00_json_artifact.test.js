const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const writeJsonArtifact = require('../lib/json_artifact');

describe('JSON artifact writer contract [00_json_artifact]', function () {
  let tempDir;

  beforeEach(function () {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-json-artifact-'));
  });

  afterEach(function () {
    fs.rmSync(tempDir, { recursive: true, force: true });
  });

  it('creates parent directories and preserves arbitrary coverage payload schemas', function () {
    const payloads = [
      { endpoints: [{ method: 'GET', path: '/api/v1/device' }], covered: 1 },
      { pages: [{ route: '/home', hitCount: 2 }], flows: [] }
    ];

    payloads.forEach((payload, index) => {
      const filePath = path.join(tempDir, `nested-${index}`, 'coverage.json');
      writeJsonArtifact(filePath, payload);

      expect(fs.readFileSync(filePath, 'utf8')).to.equal(JSON.stringify(payload, null, 2));
    });
  });

  it('propagates JSON serialization errors synchronously', function () {
    const circular = {};
    circular.self = circular;

    expect(() => writeJsonArtifact(path.join(tempDir, 'invalid.json'), circular)).to.throw(TypeError);
  });
});
