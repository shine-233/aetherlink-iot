/**
 * Structured evidence metadata for automation_tests.
 *
 * The harness uses this table to classify each test file and, when needed,
 * each test case as business, boundary, contract, catalog, config, preflight,
 * or page-smoke evidence. Metadata is classification evidence only; it does
 * not replace fresh runtime API automation or Playwright E2E results.
 */
const metadataPart01 = require("./test-metadata/part-01");
const metadataPart02 = require("./test-metadata/part-02");
const metadataPart03 = require("./test-metadata/part-03");
const metadataPart04 = require("./test-metadata/part-04");
const metadataPart05 = require("./test-metadata/part-05");
const metadataPart06 = require("./test-metadata/part-06");

const METADATA_PARTS = [
  metadataPart01,
  metadataPart02,
  metadataPart03,
  metadataPart04,
  metadataPart05,
  metadataPart06,
];

const metadataFileCounts = new Map();
for (const part of METADATA_PARTS) {
  for (const file of Object.keys(part)) {
    metadataFileCounts.set(file, (metadataFileCounts.get(file) || 0) + 1);
  }
}
const DUPLICATE_METADATA_FILES = [...metadataFileCounts.entries()]
  .filter(([, count]) => count > 1)
  .map(([file, count]) => ({ file, count }));

const TEST_METADATA = Object.assign({}, ...METADATA_PARTS);

const CASE_METADATA_BY_ID = new Map();
const DUPLICATE_CASE_IDS = [];
for (const metadata of Object.values(TEST_METADATA)) {
  for (const testCase of Array.isArray(metadata.cases) ? metadata.cases : []) {
    if (!testCase.caseId) continue;
    if (CASE_METADATA_BY_ID.has(testCase.caseId)) {
      DUPLICATE_CASE_IDS.push(testCase.caseId);
      continue;
    }
    CASE_METADATA_BY_ID.set(testCase.caseId, testCase);
  }
}
if (DUPLICATE_CASE_IDS.length > 0) {
  throw new Error(`Duplicate test metadata caseId values: ${DUPLICATE_CASE_IDS.join(", ")}`);
}

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
  return TEST_METADATA[normalized] || null;
}

function getCaseMetadata(testPath, title) {
  const metadata = getTestMetadata(testPath);
  if (!metadata) {
    return null;
  }
  return metadata.cases.find((item) => item.title === title) || null;
}

function getCaseMetadataById(caseId) {
  return typeof caseId === "string" ? CASE_METADATA_BY_ID.get(caseId) || null : null;
}

module.exports = {
  TEST_METADATA,
  DUPLICATE_METADATA_FILES,
  DUPLICATE_CASE_IDS,
  normalizeTestPath,
  getTestMetadata,
  getCaseMetadata,
  getCaseMetadataById,
};
