/**
 * Synchronously writes a JSON artifact while preserving the coverage trackers'
 * existing directory creation, formatting, encoding, and error propagation.
 *
 * @param {string} filePath
 * @param {*} payload
 */
const fs = require('fs');
const path = require('path');

function writeJsonArtifact(filePath, payload) {
  const dir = path.dirname(filePath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  fs.writeFileSync(filePath, JSON.stringify(payload, null, 2), 'utf8');
}

module.exports = writeJsonArtifact;
