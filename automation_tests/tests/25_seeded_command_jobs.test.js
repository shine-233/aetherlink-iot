/**
 * Command Jobs seeded API coverage skeleton.
 *
 * This suite exercises the customer-facing batch command workflow: preview,
 * submit, refreshable detail, support bundle, cancel, and retry. It is only
 * business evidence after it runs against a live seeded backend.
 */

const { expect } = require("chai");
const apiClient = require("../lib/api_client");
const seedData = require("../lib/seed_data");
const testData = require("../lib/test_data");
const {
  expectApiEnvelope,
  expectBlockedOrSeeded,
  expectBusinessError,
  expectSuccess,
} = require("../lib/response_assertions");

const ZERO_UUID = "00000000-0000-0000-0000-000000000000";
const COMMAND_JOB_ROWS_STATUS_FILTERS = [
  "all",
  "needs_attention",
  "retryable",
  "device_failed",
  "failed",
  "missing_log",
  "in_progress",
  "canceled",
];
const COMMAND_JOB_STATUS_COUNT_KEYS = [
  "ready",
  "dispatching",
  "submitted",
  "failed",
  "blocked",
  "canceled",
];

function commandValue() {
  return JSON.stringify(testData.getTestDryContactParams());
}

function buildCommandJobPayload(deviceId, extra = {}) {
  const subsetLimit = 10;
  return {
    scope_type: "selected_devices",
    device_ids: [deviceId],
    identify: "test_dry_contact",
    value: commandValue(),
    timeout_seconds: 60,
    subset_limit: subsetLimit,
    sample_limit: subsetLimit,
    ...extra,
  };
}

function expectNumericCountFields(data) {
  [
    "requested_count",
    "eligible_count",
    "blocked_count",
    "timeout_seconds",
  ].forEach((key) => {
    expect(data).to.have.property(key).that.is.a("number");
    expect(data[key]).to.be.at.least(0);
  });
}

function expectSupportBundleCountFields(data) {
  [
    "requested_count",
    "eligible_count",
    "blocked_count",
    "submitted_count",
    "failed_count",
    "retryable_count",
    "retry_ready_count",
    "retry_waiting_count",
    "retry_exhausted_count",
    "log_missing_count",
  ].forEach((key) => {
    expect(data).to.have.property(key).that.is.a("number");
    expect(data[key]).to.be.at.least(0);
  });
}

function expectRetryPolicyCountFields(data) {
  [
    "retry_ready_count",
    "retry_waiting_count",
    "retry_exhausted_count",
  ].forEach((key) => {
    expect(data).to.have.property(key).that.is.a("number");
    expect(data[key]).to.be.at.least(0);
  });
  expect(data.retry_ready_count).to.be.at.most(data.retryable_count || 0);
  expect(data.retry_ready_count + data.retry_waiting_count + data.retry_exhausted_count)
    .to.be.at.least(data.retryable_count || 0);
}

function expectPreviewGuidance(data) {
  expect(data).to.have.property("path_counts").that.is.an("object");
  [
    "immediate",
    "jobs",
    "blocked",
    "telemetry",
  ].forEach((key) => {
    expect(data.path_counts).to.have.property(key).that.is.a("number");
    expect(data.path_counts[key]).to.be.at.least(0);
  });
  expect(
    data.path_counts.immediate + data.path_counts.jobs + data.path_counts.blocked,
  ).to.equal(data.rows.length);
  expect(data.path_counts.telemetry).to.be.at.most(data.rows.length);
  expect(data).to.have.property("next_action").that.is.a("string").and.not.equal("");

  if (data.blocked_count > 0) {
    expect(data).to.have.property("blockers").that.is.an("array").and.not.empty;
  }
  if (Array.isArray(data.blockers)) {
    expect(data.blockers.length).to.be.at.most(5);
    data.blockers.forEach((blocker) => {
      expect(blocker).to.have.property("reason").that.is.a("string").and.not.equal("");
      expect(blocker).to.have.property("count").that.is.a("number").and.greaterThan(0);
      if (Object.prototype.hasOwnProperty.call(blocker, "advice")) {
        expect(blocker.advice).to.be.a("string");
      }
    });
  }
}

function expectCustomerHandoffEvidence(data, jobId) {
  expect(data).to.have.property("progress_health").that.is.an("object");
  expect(data.progress_health)
    .to.have.property("state")
    .that.is.oneOf([
      "running",
      "timeout_risk",
      "timed_out",
      "needs_attention",
      "canceled",
      "complete",
    ]);
  [
    "pending_count",
    "terminal_count",
    "elapsed_seconds",
    "timeout_remaining_seconds",
  ].forEach((key) => {
    expect(data.progress_health).to.have.property(key).that.is.a("number");
  });
  expect(data.progress_health.pending_count).to.be.at.least(0);
  expect(data.progress_health.terminal_count).to.be.at.least(0);
  expect(
    data.progress_health.pending_count + data.progress_health.terminal_count,
  ).to.equal(data.requested_count);
  expect(data.progress_health)
    .to.have.property("next_action")
    .that.is.a("string")
    .and.not.equal("");

  expect(data).to.have.property("handoff_summary").that.is.a("string").and.not.equal("");
  if (jobId) {
    expect(data.handoff_summary).to.include(jobId);
  }
  expect(data.handoff_summary).to.include(data.progress_health.state);
  // Governance may add a more specific execution action; the backend keeps
  // the health action in the same handoff string for audit traceability.
  expect(data.handoff_summary).to.include(data.progress_health.next_action);

  expect(data).to.have.property("audit_summary").that.is.an("object");
  expect(data.audit_summary).to.have.property("event_count").that.is.a("number").and.at.least(0);
  expect(data.audit_summary)
    .to.have.property("next_action")
    .that.is.a("string")
    .and.not.equal("");
  [
    "latest_event_type",
    "latest_event_at",
    "latest_message",
  ].forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(data.audit_summary, key)) {
      expect(data.audit_summary[key], key).to.be.a("string").and.not.equal("");
    }
  });

  expect(data).to.have.property("status_counts").that.is.an("object");
  expect(data).to.have.property("warnings").that.is.an("array").and.not.empty;
  data.warnings.forEach((warning) => {
    expect(warning).to.be.a("string").and.not.equal("");
  });
}

function expectRetryPolicyRowShape(row) {
  [
    "dispatch_attempts",
    "max_dispatch_attempts",
  ].forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(row, key)) {
      expect(row[key], key).to.be.a("number").and.at.least(0);
    }
  });
  if (Object.prototype.hasOwnProperty.call(row, "retry_state")) {
    expect(row.retry_state)
      .to.be.a("string")
      .and.not.equal("");
  }
  if (Object.prototype.hasOwnProperty.call(row, "next_retry_after")) {
    expect(row.next_retry_after).to.satisfy(
      (value) => value === null || typeof value === "string",
    );
  }
  if (row.can_retry) {
    expect(row).to.have.property("retry_state").that.is.a("string").and.not.equal("");
  }
}

function expectSupportBundleFailedDeviceDiagnostics(data) {
  const failedDevices = Array.isArray(data.failed_devices)
    ? data.failed_devices
    : [];

  failedDevices.forEach((device) => {
    expectCommandJobResponseEvidenceShape(device);
    expectRetryPolicyRowShape(device);
    expect(device).to.have.property("diagnostic_summary").that.is.an("object");
    expect(device.diagnostic_summary)
      .to.have.property("level")
      .that.is.a("string")
      .and.not.equal("");
    expect(device.diagnostic_summary)
      .to.have.property("code")
      .that.is.a("string")
      .and.not.equal("");
    expect(device.diagnostic_summary.code).to.be.oneOf([
      "device_ack_failed",
      "retryable_dispatch_failure",
      "blocked_before_dispatch",
      "canceled_before_terminal_result",
      "cancel_in_flight",
      "missing_platform_log",
      "needs_row_review",
    ]);
    expect(device.diagnostic_summary)
      .to.have.property("summary")
      .that.is.a("string")
      .and.not.equal("");
    expect(device.diagnostic_summary)
      .to.have.property("next_actions")
      .that.is.an("array")
      .and.not.empty;
    device.diagnostic_summary.next_actions.forEach((action) => {
      expect(action).to.be.a("string").and.not.equal("");
    });
    if (device.diagnostic_summary.evidence) {
      expect(device.diagnostic_summary.evidence).to.be.an("array");
      device.diagnostic_summary.evidence.forEach((evidence) => {
        expect(evidence).to.be.a("string").and.not.equal("");
      });
    }
  });
}

function expectSupportBundleCustomerEvidence(data, jobId) {
  expect(data).to.have.property("job_id", jobId);
  expect(data).to.have.property("status").that.is.a("string").and.not.equal("");
  expect(data).to.have.property("generated_at").that.is.a("string").and.not.equal("");
  expect(Date.parse(data.generated_at)).to.satisfy(
    (value) => Number.isFinite(value),
    "generated_at should be parseable evidence timestamp",
  );
  expect(data).to.have.property("status_counts").that.is.an("object");
  Object.entries(data.status_counts).forEach(([status, count]) => {
    expect(status).to.be.a("string").and.not.equal("");
    expect(count).to.be.a("number").and.at.least(0);
  });
  if (Array.isArray(data.retryable_device_ids)) {
    expect(data.retryable_device_ids.length).to.equal(data.retryable_count);
    data.retryable_device_ids.forEach((id) => {
      expect(id).to.be.a("string").and.not.equal("");
    });
  }
  if (Array.isArray(data.missing_log_device_ids)) {
    expect(data.missing_log_device_ids.length).to.equal(data.log_missing_count);
    data.missing_log_device_ids.forEach((id) => {
      expect(id).to.be.a("string").and.not.equal("");
    });
  }
  if (Array.isArray(data.events)) {
    data.events.forEach((event) => {
      expect(event).to.have.property("id").that.is.a("string").and.not.equal("");
      expect(event).to.have.property("event_type").that.is.a("string").and.not.equal("");
      if (Object.prototype.hasOwnProperty.call(event, "message")) {
        expect(event.message).to.be.a("string");
      }
    });
  }
}

function sumCommandJobStatusCounts(statusCounts) {
  return Object.values(statusCounts || {}).reduce((total, value) => total + value, 0);
}

function expectCommandJobStatusCountsMatchRequest(data) {
  if (!data.status_counts) {
    return;
  }
  expect(sumCommandJobStatusCounts(data.status_counts)).to.equal(data.requested_count);
}

function expectCommandJobStatusSnapshot(data, label) {
  expect(data, label).to.have.property("status_counts").that.is.an("object");
  COMMAND_JOB_STATUS_COUNT_KEYS.forEach((status) => {
    expect(data.status_counts, `${label}.${status}`)
      .to.have.property(status)
      .that.is.a("number")
      .and.at.least(0);
  });
  expectCommandJobStatusCountsMatchRequest(data);
}

function expectCommandJobExecutionSummary(data) {
  expect(data).to.have.property("execution_summary").that.is.an("object");
  expect(data.execution_summary)
    .to.have.property("path_type")
    .that.is.a("string")
    .and.not.equal("");
  expect(data.execution_summary)
    .to.have.property("path_label")
    .that.is.a("string")
    .and.not.equal("");
  expect(data.execution_summary)
    .to.have.property("decision")
    .that.is.a("string")
    .and.not.equal("");
  expect(data.execution_summary).to.have.property("can_close").that.is.a("boolean");
  expect(data.execution_summary)
    .to.have.property("next_action")
    .that.is.a("string")
    .and.not.equal("");
  if (Array.isArray(data.execution_summary.evidence)) {
    data.execution_summary.evidence.forEach((item) => {
      expect(item).to.be.a("string").and.not.equal("");
    });
  }
  if (Array.isArray(data.execution_summary.checklist)) {
    data.execution_summary.checklist.forEach((item) => {
      expect(item).to.have.property("key").that.is.a("string").and.not.equal("");
      expect(item).to.have.property("label").that.is.a("string").and.not.equal("");
      expect(item).to.have.property("state").that.is.a("string").and.not.equal("");
    });
  }
}

function expectCommandJobGovernanceSummary(data) {
  expect(data).to.have.property("governance_summary").that.is.an("object");
  expect(data.governance_summary)
    .to.have.property("level")
    .that.is.a("string")
    .and.not.equal("");
  expect(data.governance_summary)
    .to.have.property("title")
    .that.is.a("string")
    .and.not.equal("");
  expect(data.governance_summary)
    .to.have.property("summary")
    .that.is.a("string")
    .and.not.equal("");
  expect(data.governance_summary)
    .to.have.property("next_action")
    .that.is.a("string")
    .and.not.equal("");
  if (Array.isArray(data.governance_summary.items)) {
    data.governance_summary.items.forEach((item) => {
      expect(item).to.have.property("key").that.is.a("string").and.not.equal("");
      expect(item).to.have.property("label").that.is.a("string").and.not.equal("");
      expect(item).to.have.property("value").that.is.a("string").and.not.equal("");
      expect(item).to.have.property("state").that.is.a("string").and.not.equal("");
    });
  }
}

function expectCommandJobLifecycleOracle({ detail, summary, rowsPage, support, jobId, deviceId }) {
  [detail, summary, support].forEach((item) => {
    expect(item).to.include({ job_id: jobId });
    expect(item.requested_count).to.equal(detail.requested_count);
    expect(item.eligible_count).to.equal(detail.eligible_count);
    expect(item.blocked_count).to.equal(detail.blocked_count);
    expect(item.eligible_count + item.blocked_count).to.equal(item.requested_count);
    expectCommandJobStatusCountsMatchRequest(item);
  });

  expect(summary.rows).to.deep.equal([]);
  expect(summary.rows_truncated).to.equal(true);
  expect(summary.rows_total).to.equal(detail.rows_total);
  expect(rowsPage.total).to.equal(detail.rows_total);
  expect(rowsPage.total).to.be.at.least(rowsPage.rows.length);
  expect(rowsPage.rows.some((row) => row && row.device_id === deviceId)).to.equal(true);

  expect(support).to.include({ job_id: jobId });
  // Detail and support-bundle are separate HTTP reads. The command worker can
  // legitimately advance a row (for example ready -> submitted) between
  // those reads, so requiring byte-for-byte equality creates a timing race.
  // Validate each response as a complete, request-conserving lifecycle
  // snapshot instead; the per-device rows and job identity still have to
  // match the same persisted command job.
  expectCommandJobStatusSnapshot(detail, "detail");
  expectCommandJobStatusSnapshot(support, "support");
  expect(support).to.have.property("next_actions").that.is.an("array").and.not.empty;
  expect(support).to.have.property("share_hint").that.is.a("string").and.not.equal("");
  expectCommandJobExecutionSummary(detail);
  expectCommandJobExecutionSummary(support);
  expectCommandJobGovernanceSummary(detail);
  expectCommandJobGovernanceSummary(support);
}

function expectCanceledCommandJobSupportCloseout(data, jobId) {
  expect(data).to.include({ job_id: jobId, status: "canceled" });
  expectSupportBundleCountFields(data);
  expectSupportBundleCustomerEvidence(data, jobId);
  expectSupportBundleFailedDeviceDiagnostics(data);
  expectCommandJobExecutionSummary(data);
  expectCommandJobGovernanceSummary(data);
  expect(data).to.have.property("next_actions").that.is.an("array").and.not.empty;
  // next_actions is customer-localized text, so English words are not a
  // stable API contract.  The execution summary carries the machine-readable
  // decision and closeability that the operator UI uses across locales.
  expect(data.execution_summary).to.include({
    decision: "canceled",
    can_close: false,
  });
  const combinedActions = data.next_actions.join(" ").toLowerCase();
  expect(combinedActions).to.not.include("retry ready devices");
}

function expectCommandDeliveryLogSummary(log) {
  expect(log).to.be.an("object");
  [
    "id",
    "message_id",
    "identify",
    "status",
    "status_label",
    "created_at",
  ].forEach((key) => {
    expect(log).to.have.property(key).that.is.a("string").and.not.equal("");
  });
  expect(Date.parse(log.created_at)).to.satisfy(
    (value) => Number.isFinite(value),
    "created_at should be parseable diagnostic evidence timestamp",
  );
  [
    "data",
    "response_data",
    "error_message",
  ].forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(log, key)) {
      expect(log[key], key).to.be.a("string");
    }
  });
}

function expectCommandDeliveryDiagnostics(data, expectedDeviceId) {
  expect(data).to.be.an("object");
  expect(data).to.have.property("device_id", expectedDeviceId);
  expect(data).to.have.property("evaluated_at").that.is.a("string").and.not.equal("");
  expect(Date.parse(data.evaluated_at)).to.satisfy(
    (value) => Number.isFinite(value),
    "evaluated_at should be parseable diagnostic evidence timestamp",
  );
  expect(data).to.have.property("is_online").that.is.a("boolean");
  expect(data).to.have.property("device_status").that.is.a("number");

  expect(data).to.have.property("recent_logs").that.is.an("array");
  data.recent_logs.forEach(expectCommandDeliveryLogSummary);
  if (data.latest_log) {
    expectCommandDeliveryLogSummary(data.latest_log);
  }

  expect(data)
    .to.have.property("confirmation_channels")
    .that.is.an("array")
    .and.not.empty;
  data.confirmation_channels.forEach((channel) => {
    expect(channel).to.have.property("code").that.is.a("string").and.not.equal("");
    expect(channel).to.have.property("label").that.is.a("string").and.not.equal("");
    expect(channel).to.have.property("description").that.is.a("string").and.not.equal("");
  });

  expect(data).to.have.property("conclusion").that.is.an("object");
  expect(data.conclusion).to.have.property("level").that.is.a("string").and.not.equal("");
  expect(data.conclusion).to.have.property("code").that.is.a("string").and.not.equal("");
  expect(data.conclusion).to.have.property("summary").that.is.a("string").and.not.equal("");
  expect(data.conclusion)
    .to.have.property("next_actions")
    .that.is.an("array")
    .and.not.empty;
  data.conclusion.next_actions.forEach((action) => {
    expect(action).to.be.a("string").and.not.equal("");
  });
  if (Array.isArray(data.conclusion.evidence)) {
    data.conclusion.evidence.forEach((item) => {
      expect(item).to.be.a("string").and.not.equal("");
    });
  }
}

function expectCommandJobResponseEvidenceShape(row) {
  expect(row).to.be.an("object");
  [
    "response_status",
    "response_status_label",
    "response_data",
    "response_error",
    "response_at",
    "command_log_created_at",
  ].forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(row, key)) {
      expect(row[key], key).to.satisfy(
        (value) => value === null || typeof value === "string",
      );
    }
  });
  if (Object.prototype.hasOwnProperty.call(row, "response_recorded")) {
    expect(row.response_recorded).to.be.a("boolean");
    if (row.response_recorded) {
      const hasEvidence = [
        "response_status",
        "response_status_label",
        "response_data",
        "response_error",
        "response_at",
        "command_log_created_at",
      ].some((key) => typeof row[key] === "string" && row[key] !== "");
      expect(
        hasEvidence,
        "response_recorded=true must include response evidence",
      ).to.equal(true);
    }
  }
}

function commandJobRowMatchesStatusFilter(row, statusFilter) {
  switch (statusFilter) {
    case "all":
      return true;
    case "needs_attention":
      return (
        row.can_retry ||
        row.status === "failed" ||
        row.status === "blocked" ||
        row.status === "canceled" ||
        (row.status === "submitted" && row.log_recorded === false) ||
        row.response_status === "4" ||
        row.response_status_label === "device_ack_failed"
      );
    case "retryable":
      return row.status === "failed" && row.can_retry === true;
    case "failed":
      return (
        row.status === "failed" ||
        row.status === "blocked" ||
        row.response_status === "4" ||
        row.response_status_label === "device_ack_failed"
      );
    case "device_failed":
      return (
        row.response_status === "4" ||
        row.response_status_label === "device_ack_failed"
      );
    case "missing_log":
      return row.status === "submitted" && row.log_recorded === false;
    case "in_progress":
      return ["ready", "dispatching", "submitted"].includes(row.status);
    case "canceled":
      return row.status === "canceled";
    default:
      return false;
  }
}

function expectCommandJobRowShape(row) {
  expect(row).to.be.an("object");
  expect(row).to.have.property("device_id").that.is.a("string").and.not.equal("");
  expect(row).to.have.property("eligible").that.is.a("boolean");
  expect(row).to.have.property("status").that.is.a("string").and.not.equal("");
  expect(row).to.have.property("can_retry").that.is.a("boolean");
  expectCommandJobResponseEvidenceShape(row);
  expectRetryPolicyRowShape(row);
}

function expectCommandJobListItemShape(row) {
  expect(row).to.be.an("object");
  expect(row).to.have.property("job_id").that.is.a("string").and.not.equal("");
  expect(row).to.have.property("scope_type").that.is.a("string").and.not.equal("");
  expect(row).to.have.property("identify").that.is.a("string");
  expect(row).to.have.property("status").that.is.a("string").and.not.equal("");
  expectSupportBundleCountFields(row);
  expect(row).to.have.property("needs_operator_action").that.is.a("boolean");
  expect(row)
    .to.have.property("needs_operator_action_count")
    .that.is.a("number")
    .and.at.least(0);
  expect(row).to.have.property("can_cancel").that.is.a("boolean");
  expect(row).to.have.property("can_retry_failed").that.is.a("boolean");
}

function expectPreviewResult(data, deviceId) {
  expect(data).to.be.an("object");
  expect(data).to.include({
    scope_type: "selected_devices",
  });
  expect(data)
    .to.have.property("preview_token")
    .that.is.a("string")
    .and.not.equal("");
  expectNumericCountFields(data);
  expect(data).to.have.property("rows").that.is.an("array").and.not.empty;
  expect(data.rows.some((row) => row && row.device_id === deviceId)).to.equal(
    true,
  );
  expect(data.requested_count).to.equal(1);
  expect(data.eligible_count + data.blocked_count).to.equal(
    data.requested_count,
  );
  expectPreviewGuidance(data);
}

function expectJobResult(data, jobId) {
  expect(data).to.be.an("object");
  expect(data).to.have.property("job_id").that.is.a("string").and.not.equal("");
  if (jobId) {
    expect(data.job_id).to.equal(jobId);
  }
  expect(data).to.include({
    scope_type: "selected_devices",
  });
  expect(data).to.have.property("status").that.is.a("string").and.not.equal("");
  expectNumericCountFields(data);
  [
    "submitted_count",
    "failed_count",
    "retryable_count",
    "log_missing_count",
  ].forEach((key) => {
    expect(data).to.have.property(key).that.is.a("number");
    expect(data[key]).to.be.at.least(0);
  });
  expectRetryPolicyCountFields(data);
  expect(data).to.have.property("can_cancel").that.is.a("boolean");
  expect(data).to.have.property("can_retry_failed").that.is.a("boolean");
  expectCustomerHandoffEvidence(data, jobId || data.job_id);
}

function expectJobRows(data, deviceId) {
  expect(data).to.have.property("rows").that.is.an("array");
  expect(data.rows_total).to.be.a("number").and.at.least(data.rows.length);
  data.rows.forEach(expectCommandJobRowShape);
  expect(data.rows.some((row) => row && row.device_id === deviceId)).to.equal(
    true,
  );
}

function expectRowsResult(data, {
  deviceId,
  statusFilter,
  search,
  requireDeviceMatch = true,
  expectEmpty = false,
}) {
  expect(data).to.be.an("object");
  expect(data).to.include({
    page: 1,
  });
  expect(data).to.have.property("page_size").that.is.a("number");
  expect(data).to.have.property("total").that.is.a("number").and.at.least(0);
  expect(data).to.have.property("rows").that.is.an("array");
  expect(data).to.have.property("rows_truncated").that.is.a("boolean");
  if (statusFilter) {
    expect(data).to.have.property("status_filter", statusFilter);
  }
  if (search) {
    const normalizedSearch = Array.from(String(search).trim()).slice(0, 64).join("");
    expect(data).to.have.property("search", normalizedSearch);
  }
  data.rows.forEach(expectCommandJobRowShape);
  data.rows.forEach((row) => {
    expect(
      commandJobRowMatchesStatusFilter(row, statusFilter || "all"),
      "row should match " + (statusFilter || "all") + " filter",
    ).to.equal(true);
  });
  if (expectEmpty) {
    expect(data.total).to.equal(0);
    expect(data.rows).to.deep.equal([]);
  } else if (requireDeviceMatch) {
    expect(data.rows.some((row) => row && row.device_id === deviceId)).to.equal(
      true,
    );
  }
}

function commandJobRowSearchCandidates(rows, fallbackDeviceId) {
  const candidates = [
    { field: "device_id", value: fallbackDeviceId },
  ];
  const firstRow = rows.find((row) => row && row.device_id === fallbackDeviceId) || rows[0] || {};
  [
    "device_number",
    "name",
    "message_id",
    "response_error",
    "response_data",
    "reason",
    "advice",
  ].forEach((field) => {
    if (typeof firstRow[field] === "string" && firstRow[field].trim()) {
      candidates.push({ field, value: firstRow[field].trim() });
    }
  });
  return candidates;
}

function expectRejected(resp, textPattern, allowedCodes) {
  expectApiEnvelope(resp);
  expect(resp.code).to.be.oneOf(allowedCodes);
  expect(resp.message).to.be.a("string").and.not.equal("");
  if (textPattern) {
    expect(resp.message.toLowerCase()).to.match(textPattern);
  }
}

async function createSeededCommandJob(deviceId) {
  const previewResp = await apiClient.post(
    "/command/datas/jobs/preview",
    buildCommandJobPayload(deviceId),
    "tenant_admin",
  );
  expectSuccess(previewResp);
  expectPreviewResult(previewResp.data, deviceId);

  const submitResp = await apiClient.post(
    "/command/datas/jobs/submit?include_rows=true",
    buildCommandJobPayload(deviceId, {
      preview_token: previewResp.data.preview_token,
    }),
    "tenant_admin",
  );
  expectSuccess(submitResp);
  expectJobResult(submitResp.data);
  expectJobRows(submitResp.data, deviceId);
  return submitResp.data;
}

describe("Seeded command job API coverage [25_seeded_command_jobs]", function () {
  this.timeout(45000);

  let seededDevice = null;
  let deviceId = "";

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error(
        "Backend service is not running locally for 25_seeded_command_jobs.test.js; Command Jobs API coverage requires a healthy API service",
      );
    }

    await apiClient.login("tenant_admin");
    seededDevice = await seedData.ensureDevice("tenant_admin");
    expectBlockedOrSeeded(seededDevice, "command job seeded device");
    deviceId = seededDevice.id;
  });

  after(async function () {
    try {
      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }
    } finally {
      apiClient.clearAllTokens();
    }
  });

  it("rejects command job preview requests without an identify value", async function () {
    const previewResp = await apiClient.post(
      "/command/datas/jobs/preview",
      buildCommandJobPayload(deviceId, { identify: "" }),
      "tenant_admin",
    );

    expectRejected(previewResp, /identify|required/, [100002]);
  });

  it("rejects unsupported command job scope types before previewing devices", async function () {
    const previewResp = await apiClient.post(
      "/command/datas/jobs/preview",
      buildCommandJobPayload(deviceId, { scope_type: "all_devices" }),
      "tenant_admin",
    );

    expectRejected(previewResp, /scope_type|unsupported/, [100002]);
  });

  it("previews a seeded selected-device command job without dispatching it", async function () {
    const previewResp = await apiClient.post(
      "/command/datas/jobs/preview",
      buildCommandJobPayload(deviceId),
      "tenant_admin",
    );

    expectSuccess(previewResp);
    expectPreviewResult(previewResp.data, deviceId);
  });

  it("requires a preview token before submitting a command job", async function () {
    const submitResp = await apiClient.post(
      "/command/datas/jobs/submit",
      buildCommandJobPayload(deviceId),
      "tenant_admin",
    );

    expectBusinessError(submitResp, 100002, "preview token");
  });

  it("rejects stale command job preview tokens without persisting a job", async function () {
    const previewResp = await apiClient.post(
      "/command/datas/jobs/preview",
      buildCommandJobPayload(deviceId),
      "tenant_admin",
    );
    expectSuccess(previewResp);
    expectPreviewResult(previewResp.data, deviceId);

    const submitResp = await apiClient.post(
      "/command/datas/jobs/submit",
      buildCommandJobPayload(deviceId, {
        preview_token: previewResp.data.preview_token + "-stale",
      }),
      "tenant_admin",
    );

    expectBusinessError(submitResp, 100002, "preview expired");
  });

  it("returns a stable paged command job history shape", async function () {
    const listResp = await apiClient.get(
      "/command/datas/jobs",
      { page: 1, page_size: 20 },
      "tenant_admin",
    );

    expectSuccess(listResp);
    expect(listResp.data).to.be.an("object");
    expect(listResp.data).to.have.property("total").that.is.a("number");
    expect(listResp.data.total).to.be.at.least(0);
    expect(listResp.data).to.have.property("list").that.is.an("array");
    listResp.data.list.forEach(expectCommandJobListItemShape);

    const attentionResp = await apiClient.get(
      "/command/datas/jobs",
      {
        page: 1,
        page_size: 20,
        attention_filter: "needs_operator_action",
      },
      "tenant_admin",
    );
    expectSuccess(attentionResp);
    expect(attentionResp.data).to.have.property("list").that.is.an("array");
    attentionResp.data.list.forEach((row) => {
      expectCommandJobListItemShape(row);
      expect(row.needs_operator_action).to.equal(true);
      expect(row.needs_operator_action_count).to.be.greaterThan(0);
    });
  });

  it("creates, lists, updates, and deletes an owned fleet saved filter", async function () {
    const name = `codex-saved-filter-${Date.now()}`;
    let filterId = "";
    const payload = {
      name,
      device_filter: { search: "codex-endpoint-coverage" },
      preview_total: 0,
      shared: false,
    };

    try {
      const createResp = await apiClient.post(
        "/command/datas/saved-filters",
        payload,
        "tenant_admin",
      );
      expectSuccess(createResp);
      expect(createResp.data).to.be.an("object");
      expect(createResp.data).to.include({ name, owned: true });
      expect(createResp.data.id).to.be.a("string").and.not.equal("");
      filterId = createResp.data.id;
      expect(createResp.data.device_filter).to.deep.equal(payload.device_filter);

      const listResp = await apiClient.get(
        "/command/datas/saved-filters",
        {},
        "tenant_admin",
      );
      expectSuccess(listResp);
      expect(listResp.data).to.have.property("list").that.is.an("array");
      expect(listResp.data.list.some((item) => item.id === filterId)).to.equal(true);

      const updateResp = await apiClient.put(
        `/command/datas/saved-filters/${filterId}`,
        {
          ...payload,
          name: `${name}-updated`,
          device_filter: { search: "codex-endpoint-coverage-updated" },
        },
        "tenant_admin",
      );
      expectSuccess(updateResp);
      expect(updateResp.data).to.include({
        id: filterId,
        name: `${name}-updated`,
        owned: true,
      });
      expect(updateResp.data.device_filter).to.deep.equal({
        search: "codex-endpoint-coverage-updated",
      });
    } finally {
      if (filterId) {
        const deleteResp = await apiClient.delete(
          `/command/datas/saved-filters/${filterId}`,
          {},
          "tenant_admin",
        );
        expectSuccess(deleteResp);
      }
    }
  });

  it("returns seeded command delivery diagnostics with operator next actions", async function () {
    const diagnosticsResp = await apiClient.get(
      "/command/datas/delivery/diagnostics/" + deviceId,
      { limit: 5 },
      "tenant_admin",
    );

    expectSuccess(diagnosticsResp);
    expectCommandDeliveryDiagnostics(diagnosticsResp.data, deviceId);
  });

  it("keeps invalid command job detail and support boundaries explicit", async function () {
    const detailResp = await apiClient.get(
      "/command/datas/jobs/" + ZERO_UUID,
      {},
      "tenant_admin",
    );
    expectRejected(detailResp, /record|not found|job|command/, [100000, 404]);

    const supportResp = await apiClient.get(
      "/command/datas/jobs/" + ZERO_UUID + "/support-bundle",
      {},
      "tenant_admin",
    );
    expectRejected(supportResp, /record|not found|job|command/, [100000, 404]);
  });

  it("filters and searches command job rows with response evidence shape", async function () {
    const submittedJob = await createSeededCommandJob(deviceId);
    const jobId = submittedJob.job_id;

    for (const statusFilter of COMMAND_JOB_ROWS_STATUS_FILTERS) {
      const filteredRowsResp = await apiClient.get(
        "/command/datas/jobs/" + jobId + "/rows",
        {
          page: 1,
          page_size: 50,
          status_filter: statusFilter,
        },
        "tenant_admin",
      );
      expectSuccess(filteredRowsResp);
      expectRowsResult(filteredRowsResp.data, {
        deviceId,
        statusFilter,
        requireDeviceMatch: statusFilter === "all",
      });
    }

    const allRowsResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/rows",
      {
        page: 1,
        page_size: 50,
        status_filter: "all",
      },
      "tenant_admin",
    );
    expectSuccess(allRowsResp);
    expectRowsResult(allRowsResp.data, {
      deviceId,
      statusFilter: "all",
    });

    for (const candidate of commandJobRowSearchCandidates(
      allRowsResp.data.rows,
      deviceId,
    )) {
      const searchedRowsResp = await apiClient.get(
        "/command/datas/jobs/" + jobId + "/rows",
        {
          page: 1,
          page_size: 50,
          status_filter: "all",
          search: candidate.value,
        },
        "tenant_admin",
      );
      expectSuccess(searchedRowsResp);
      expectRowsResult(searchedRowsResp.data, {
        deviceId,
        statusFilter: "all",
        search: candidate.value,
      });
    }

    const noMatchRowsResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/rows",
      {
        page: 1,
        page_size: 50,
        status_filter: "all",
        search: ZERO_UUID,
      },
      "tenant_admin",
    );
    expectSuccess(noMatchRowsResp);
    expectRowsResult(noMatchRowsResp.data, {
      deviceId,
      statusFilter: "all",
      search: ZERO_UUID,
      expectEmpty: true,
    });

    const cancelResp = await apiClient.post(
      "/command/datas/jobs/" + jobId + "/cancel",
      {},
      "tenant_admin",
    );
    expectSuccess(cancelResp);
    expectJobResult(cancelResp.data, jobId);
  });

  it("previews, submits, refreshes, and packages a seeded command job", async function () {
    const submittedJob = await createSeededCommandJob(deviceId);
    const jobId = submittedJob.job_id;

    const listResp = await apiClient.get(
      "/command/datas/jobs",
      { page: 1, page_size: 20 },
      "tenant_admin",
    );
    expectSuccess(listResp);
    expect(listResp.data).to.be.an("object");
    expect(listResp.data).to.have.property("total").that.is.a("number");
    expect(listResp.data).to.have.property("list").that.is.an("array");
    listResp.data.list.forEach(expectCommandJobListItemShape);
    expect(
      listResp.data.list.some((row) => row && row.job_id === jobId),
    ).to.equal(true);

    const detailResp = await apiClient.get(
      "/command/datas/jobs/" + jobId,
      { include_rows: true },
      "tenant_admin",
    );
    expectSuccess(detailResp);
    expectJobResult(detailResp.data, jobId);
    expectJobRows(detailResp.data, deviceId);

    const summaryResp = await apiClient.get(
      "/command/datas/jobs/" + jobId,
      { include_rows: false },
      "tenant_admin",
    );
    expectSuccess(summaryResp);
    expectJobResult(summaryResp.data, jobId);
    expect(summaryResp.data.rows).to.be.an("array").and.empty;
    expect(summaryResp.data.rows_truncated).to.equal(true);
    expect(summaryResp.data.rows_total).to.be.a("number").and.at.least(1);

    const rowsResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/rows",
      { page: 1, page_size: 200 },
      "tenant_admin",
    );
    expectSuccess(rowsResp);
    expectRowsResult(rowsResp.data, {
      deviceId,
      statusFilter: "all",
    });
    expect(rowsResp.data).to.have.property("page_size", 200);

    const secondPageRowsResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/rows",
      {
        page: 2,
        page_size: 1,
        status_filter: "all",
      },
      "tenant_admin",
    );
    expectSuccess(secondPageRowsResp);
    expect(secondPageRowsResp.data).to.include({
      page: 2,
      page_size: 1,
      status_filter: "all",
    });
    expect(secondPageRowsResp.data).to.have.property("total").that.equals(rowsResp.data.total);
    expect(secondPageRowsResp.data).to.have.property("rows").that.is.an("array");
    expect(secondPageRowsResp.data).to.have.property("rows_truncated").that.is.a("boolean");
    secondPageRowsResp.data.rows.forEach(expectCommandJobRowShape);

    const searchedRowsResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/rows",
      {
        page: 1,
        page_size: 50,
        status_filter: "all",
        search: deviceId,
      },
      "tenant_admin",
    );
    expectSuccess(searchedRowsResp);
    expectRowsResult(searchedRowsResp.data, {
      deviceId,
      statusFilter: "all",
      search: deviceId,
    });

    const noMatchRowsResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/rows",
      {
        page: 1,
        page_size: 50,
        status_filter: "all",
        search: ZERO_UUID,
      },
      "tenant_admin",
    );
    expectSuccess(noMatchRowsResp);
    expectRowsResult(noMatchRowsResp.data, {
      deviceId,
      statusFilter: "all",
      search: ZERO_UUID,
      expectEmpty: true,
    });

    const supportResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/support-bundle",
      {},
      "tenant_admin",
    );
    expectSuccess(supportResp);
    expect(supportResp.data).to.include({
      job_id: jobId,
      scope_type: "selected_devices",
      identify: "test_dry_contact",
    });
    expectSupportBundleCountFields(supportResp.data);
    expectRetryPolicyCountFields(supportResp.data);
    expectSupportBundleCustomerEvidence(supportResp.data, jobId);
    expectSupportBundleFailedDeviceDiagnostics(supportResp.data);
    expect(supportResp.data)
      .to.have.property("next_actions")
      .that.is.an("array")
      .and.not.empty;
    expect(supportResp.data)
      .to.have.property("share_hint")
      .that.is.a("string")
      .and.not.equal("");
    expectCommandJobLifecycleOracle({
      detail: detailResp.data,
      summary: summaryResp.data,
      rowsPage: rowsResp.data,
      support: supportResp.data,
      jobId,
      deviceId,
    });

    const cancelResp = await apiClient.post(
      "/command/datas/jobs/" + jobId + "/cancel",
      {},
      "tenant_admin",
    );
    expectSuccess(cancelResp);
    expectJobResult(cancelResp.data, jobId);

    // A blocked-only job can be finalized by the dispatch worker before the
    // cancel request reaches the API. The endpoint must preserve that terminal
    // state; only an active job is expected to transition to canceled.
    if (cancelResp.data.status !== "canceled") {
      expect(cancelResp.data.status).to.be.oneOf([
        "failed",
        "partially_failed",
        "completed",
      ]);
      expect(cancelResp.data.can_cancel).to.equal(false);
      const terminalSupportResp = await apiClient.get(
        "/command/datas/jobs/" + jobId + "/support-bundle",
        {},
        "tenant_admin",
      );
      expectSuccess(terminalSupportResp);
      expectSupportBundleCustomerEvidence(terminalSupportResp.data, jobId);
      return;
    }

    const canceledDetailResp = await apiClient.get(
      "/command/datas/jobs/" + jobId,
      { include_rows: false },
      "tenant_admin",
    );
    expectSuccess(canceledDetailResp);
    expectJobResult(canceledDetailResp.data, jobId);
    if (canceledDetailResp.data.status !== "canceled") {
      expect(canceledDetailResp.data.status).to.be.oneOf([
        "failed",
        "partially_failed",
        "completed",
      ]);
      expect(canceledDetailResp.data.can_cancel).to.equal(false);
      const racedSupportResp = await apiClient.get(
        "/command/datas/jobs/" + jobId + "/support-bundle",
        {},
        "tenant_admin",
      );
      expectSuccess(racedSupportResp);
      expectSupportBundleCustomerEvidence(racedSupportResp.data, jobId);
      return;
    }
    expect(canceledDetailResp.data).to.have.property("status", "canceled");
    expect(canceledDetailResp.data.rows).to.be.an("array").and.empty;

    const canceledSupportResp = await apiClient.get(
      "/command/datas/jobs/" + jobId + "/support-bundle",
      {},
      "tenant_admin",
    );
    expectSuccess(canceledSupportResp);
    expectCanceledCommandJobSupportCloseout(canceledSupportResp.data, jobId);

    const retryResp = await apiClient.post(
      "/command/datas/jobs/" + jobId + "/retry",
      {},
      "tenant_admin",
    );
    expectSuccess(retryResp);
    expectJobResult(retryResp.data, jobId);
  });
});
