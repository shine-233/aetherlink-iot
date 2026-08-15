/**
 * Structured evidence metadata for automation_tests.
 *
 * The harness uses this table to classify each test file and, when needed,
 * each test case as business, boundary, contract, catalog, config, preflight,
 * or page-smoke evidence. Metadata is classification evidence only; it does
 * not replace fresh runtime API automation or Playwright E2E results.
 */
const path = require("path");

const metadataPart01 = require("./test-metadata/part-01");
const metadataPart02 = require("./test-metadata/part-02");
const metadataPart03 = require("./test-metadata/part-03");
const metadataPart04 = require("./test-metadata/part-04");
const metadataPart05 = require("./test-metadata/part-05");
const metadataPart06 = require("./test-metadata/part-06");

const TEST_METADATA = {
  ...metadataPart01,
  ...metadataPart02,
  ...metadataPart03,
  ...metadataPart04,
  ...metadataPart05,
  ...metadataPart06,
};

function normalizeTestPath(testPath) {
  const source =
    typeof testPath === "string"
      ? testPath
      : testPath && typeof testPath === "object"
        ? testPath.file
        : "";
  if (!source) {
    return "";
  }
  return source.replace(/\\/g, "/").replace(/^\.\//, "");
}

function getTestMetadata(testPath) {
  const normalized = normalizeTestPath(testPath);
  if (!normalized) {
    return null;
  }
  if (TEST_METADATA[normalized]) {
    return TEST_METADATA[normalized];
  }
  const basename = path.posix.basename(normalized);
  return (
    Object.values(TEST_METADATA).find(
      (item) => path.posix.basename(item.file) === basename,
    ) || null
  );
}

function getCaseMetadata(testPath, title) {
  const metadata = getTestMetadata(testPath);
  if (!metadata) {
    return null;
  }
  return metadata.cases.find((item) => item.title === title) || null;
}

module.exports = {
  TEST_METADATA,
  normalizeTestPath,
  getTestMetadata,
  getCaseMetadata,
};
