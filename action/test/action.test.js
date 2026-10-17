"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { inputs, commandArguments, parseReport, markdown, run } = require("../src/index");
const { version } = require("../package.json");

function fixture(t) { const root = fs.mkdtempSync(path.join(os.tmpdir(), "sanad-action-")); t.after(() => fs.rmSync(root, { recursive: true, force: true })); return root; }
const report = (mode) => ({ version: 1, command: mode, success: true, changed: false, verification: "online", findings: [], files: [], changes: [], decisions: [] });

test("explicit modes, host-aware identity, and write controls", (t) => {
  const root = fixture(t); const env = { GITHUB_WORKSPACE: root, GITHUB_REPOSITORY: "a/b", GITHUB_SERVER_URL: "https://example.ghe.com" };
  const values = inputs((name) => ({ mode: "update", write: "true" })[name] || "", env);
  assert.equal(values.repo, "example.ghe.com/a/b");
  assert.ok(commandArguments(values).includes("--write"));
  assert.throws(() => inputs((name) => ({ mode: "check", write: "true" })[name] || "", env), /write requires/);
  assert.throws(() => inputs((name) => name === "mode" ? "apply" : "", env), /mode must/);
  assert.throws(() => inputs((name) => name === "config" ? "../other" : "", env), /config must stay/);
  assert.throws(() => inputs((name) => name === "working-directory" ? ".." : "", env), /must stay/);
});

test("report paths and versions are validated", () => {
  const r = report("plan"); assert.equal(parseReport(JSON.stringify(r), "plan").success, true);
  assert.throws(() => parseReport(JSON.stringify({ ...r, version: 2 }), "plan"), /unsupported/);
  assert.throws(() => parseReport(JSON.stringify({ ...r, files: [{ path: "../secret" }] }), "plan"), /unsafe/);
  assert.throws(() => parseReport(JSON.stringify({ ...r, files: [{ path: "/tmp/secret" }] }), "plan"), /unsafe/);
});

test("summaries include transitive changes and escape upstream text", () => {
  const r = report("plan"); r.findings = [{ severity: "warning", message: "<script>|upstream" }];
  r.changes = [{ key: "a/nested@v1", after: { commit: "sha1-new", workflows: [".github/workflows/ci.yml"] } }];
  const text = markdown(r); assert.ok(text.includes("a/nested@v1")); assert.ok(text.includes("sha1-new")); assert.ok(text.includes("&lt;script&gt;&#124;"));
});

test("Action keeps preview paths out of staging outputs and retains operational reports", async (t) => {
  const root = fixture(t); const old = process.env.GITHUB_WORKSPACE; process.env.GITHUB_WORKSPACE = root;
  t.after(() => { if (old === undefined) delete process.env.GITHUB_WORKSPACE; else process.env.GITHUB_WORKSPACE = old; });
  const outputs = {}; const failures = [];
  const api = { getInput: (name) => name === "mode" ? "plan" : "", setSecret() {}, addPath() {}, setOutput: (k, v) => { outputs[k] = v; }, warning() {}, error() {}, setFailed: (s) => failures.push(s), summary: { addRaw() { return this; }, async write() {} } };
  const r = report("plan"); r.files = [{ path: ".github/workflows/actions.lock" }];
  await run({ core: api, install: async () => path.join(root, "sanad"), exec: async (_, args) => ({ exitCode: 0, stdout: JSON.stringify(args[0] === "version" ? { version } : r) }) });
  assert.equal(outputs["changed-files"], "[]"); assert.equal(outputs.passed, "true"); assert.equal(failures.length, 0);
  assert.ok(fs.existsSync(outputs["report-path"]));
  const bad = { ...r, success: false, findings: [{ code: "auth", severity: "error", message: "missing token" }] };
  await run({ core: api, install: async () => path.join(root, "sanad"), exec: async (_, args) => ({ exitCode: args[0] === "version" ? 0 : 2, stdout: JSON.stringify(args[0] === "version" ? { version } : bad) }) });
  assert.equal(outputs.passed, "false"); assert.equal(failures.length, 1);
});
