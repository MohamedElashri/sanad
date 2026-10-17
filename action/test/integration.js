"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");
const { run } = require("../src/index");
const { version } = require("../package.json");

async function main() {
  const binary = process.env.SANAD_ACTION_BINARY;
  if (!binary || !process.env.SANAD_BACKEND) throw new Error("SANAD_ACTION_BINARY and SANAD_BACKEND are required");
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "sanad-action-integration-"));
  try {
    const workflows = path.join(dir, ".github/workflows"); fs.mkdirSync(workflows, { recursive: true });
    fs.writeFileSync(path.join(workflows, "ci.yml"), "name: test\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo test\n");
    process.env.GH_TOKEN = "native-contract-test"; process.env.GITHUB_REPOSITORY = "example/test"; process.env.GITHUB_WORKSPACE = dir;
    process.env.SANAD_ACTION_TESTING = "true";
    execFileSync(binary, ["init", "--root", dir, "--repo", "example/test", "--confirm-native", "--write"], { env: process.env, stdio: "pipe" });
    for (const mode of ["check", "status", "plan", "sync", "update", "setup"]) {
      const outputs = {}; const failures = [];
      const api = { getInput: (name) => ({ mode, token: "native-contract-test" })[name] || "", setSecret() {}, addPath() {}, setOutput: (k, v) => { outputs[k] = v; }, warning() {}, error() {}, setFailed: (s) => failures.push(s), summary: { addRaw() { return this; }, async write() {} } };
      await run({ core: api }); assert.deepEqual(failures, [], `${mode} failed`); assert.equal(outputs.passed, "true"); assert.equal(outputs["sanad-version"], version);
    }
    const files = {};
    for (const name of ["GITHUB_OUTPUT", "GITHUB_PATH", "GITHUB_STEP_SUMMARY"]) {
      files[name] = path.join(dir, name); fs.writeFileSync(files[name], "");
    }
    execFileSync(process.execPath, [path.resolve(__dirname, "../dist/index.js")], {
      env: { ...process.env, ...files, INPUT_MODE: "check", INPUT_TOKEN: "native-contract-test", INPUT_WRITE: "false", "INPUT_WORKING-DIRECTORY": "." },
      stdio: "pipe",
    });
    assert.match(fs.readFileSync(files.GITHUB_OUTPUT, "utf8"), /passed<<[^\r\n]+\r?\ntrue\r?\n/);
    assert.ok(fs.statSync(files.GITHUB_STEP_SUMMARY).size > 0);
    console.log("Action integration passed for all native modes and the distribution bundle");
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
}
main().catch((error) => { console.error(error); process.exitCode = 1; });
