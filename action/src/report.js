"use strict";

const core = require("@actions/core");

const MAX_ANNOTATIONS = 50;

function parseReport(stdout, expectedVersion) {
  let report;
  try {
    report = JSON.parse(stdout);
  } catch (error) {
    throw new Error(`Sanad did not produce valid JSON: ${error.message}`);
  }
  if (report.version !== expectedVersion) {
    throw new Error(`unsupported Sanad report version ${report.version}; expected ${expectedVersion}`);
  }
  return report;
}

function annotateCheck(report) {
  const violations = Array.isArray(report.violations) ? report.violations : [];
  for (const violation of violations.slice(0, MAX_ANNOTATIONS)) {
    core.error(violation.reason || violation.reason_code || "Sanad policy violation", {
      title: `Sanad: ${violation.action || violation.decision}`,
      file: violation.file,
      startLine: violation.line,
      startColumn: violation.column,
    });
  }
  if (violations.length > MAX_ANNOTATIONS) {
    core.warning(`${violations.length - MAX_ANNOTATIONS} additional Sanad violations are available in the JSON report`);
  }
}

function reportMetrics(mode, report) {
  if (mode === "check") {
    return { violations: report.summary.violations, updates: report.summary.updates, pending: report.summary.pending_cooldown };
  }
  if (mode === "plan" || mode === "apply") {
    return { violations: report.summary.policy_violations, updates: report.summary.updates_available, pending: report.summary.pending_cooldown };
  }
  return { violations: report.summary.blocked, updates: report.summary.updates, pending: report.summary.pending_cooldown };
}

function annotateDecisions(mode, report) {
  if (mode === "check") return annotateCheck(report);
  const entries = mode === "upgrade" ? report.actions : report.files.flatMap((file) => file.actions.map((action) => ({ ...action, file: file.path })));
  const problems = entries.filter((entry) => String(entry.decision).startsWith("error"));
  for (const entry of problems.slice(0, MAX_ANNOTATIONS)) {
    const properties = { title: `Sanad: ${entry.action || entry.raw || entry.decision}`, file: entry.file, startLine: entry.line };
    const message = entry.reason || entry.reason_code || entry.decision;
    if (mode === "plan") core.warning(message, properties);
    else core.error(message, properties);
  }
  if (problems.length > MAX_ANNOTATIONS) core.warning(`${problems.length - MAX_ANNOTATIONS} additional Sanad findings are available in the JSON report`);
}

async function summarizeCheck(report, version) {
  const summary = report.summary;
  core.summary
    .addHeading("Sanad check", 2)
    .addRaw(`Sanad ${version} ${report.passed ? "passed" : "found policy violations"}.`)
    .addTable([
      [{ data: "Checked", header: true }, { data: "Violations", header: true }, { data: "Updates", header: true }, { data: "Pending cooldown", header: true }, { data: "Skipped", header: true }],
      [String(summary.checked), String(summary.violations), String(summary.updates), String(summary.pending_cooldown), String(summary.skipped)],
    ]);
  await core.summary.write();
}

async function summarizeReport(mode, report, version, passed) {
  if (mode === "check") return summarizeCheck(report, version);
  const metrics = reportMetrics(mode, report);
  core.summary
    .addHeading(`Sanad ${mode}`, 2)
    .addRaw(`Sanad ${version} ${passed ? "completed" : "failed"}.`)
    .addTable([
      [{ data: "Updates", header: true }, { data: "Pending cooldown", header: true }, { data: mode === "upgrade" ? "Blocked" : "Policy violations", header: true }],
      [String(metrics.updates), String(metrics.pending), String(metrics.violations)],
    ]);
  await core.summary.write();
}

module.exports = { MAX_ANNOTATIONS, annotateCheck, annotateDecisions, parseReport, reportMetrics, summarizeCheck, summarizeReport };
