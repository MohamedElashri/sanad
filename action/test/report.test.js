"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const { parseReport, reportMetrics } = require("../src/report");

test("parseReport accepts the expected contract version", () => {
  assert.deepEqual(parseReport('{"version":1,"passed":true}', 1), { version: 1, passed: true });
});

test("reportMetrics normalizes action output counts", () => {
  assert.deepEqual(reportMetrics("check", { summary: { violations: 1, updates: 2, pending_cooldown: 3 } }), { violations: 1, updates: 2, pending: 3 });
  assert.deepEqual(reportMetrics("apply", { summary: { policy_violations: 4, updates_available: 5, pending_cooldown: 6 } }), { violations: 4, updates: 5, pending: 6 });
  assert.deepEqual(reportMetrics("upgrade", { summary: { blocked: 7, updates: 8, pending_cooldown: 9 } }), { violations: 7, updates: 8, pending: 9 });
});

test("parseReport rejects invalid JSON and unknown versions", () => {
  assert.throws(() => parseReport("not-json", 1), /valid JSON/);
  assert.throws(() => parseReport('{"version":2}', 1), /unsupported Sanad report version/);
});
