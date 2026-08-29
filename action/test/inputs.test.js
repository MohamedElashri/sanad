"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { parseBoolean, resolveConfigPath, resolveWorkingDirectory, validateInputs } = require("../src/inputs");

test("parseBoolean accepts only explicit booleans", () => {
  assert.equal(parseBoolean("TRUE", "fresh"), true);
  assert.equal(parseBoolean(" false ", "fresh"), false);
  assert.throws(() => parseBoolean("yes", "fresh"), /must be/);
});

test("validateInputs rejects mode-specific input combinations", () => {
  assert.throws(() => validateInputs({ mode: "plan", fresh: true, strict: false, write: false }), /only with mode=check/);
  assert.throws(() => validateInputs({ mode: "check", fresh: false, strict: false, write: true }), /only with mode=apply or mode=upgrade/);
  assert.throws(() => validateInputs({ mode: "shell", fresh: false, strict: false, write: false }), /mode must be/);
  assert.equal(validateInputs({ mode: "setup", fresh: false, strict: false, write: false }).mode, "setup");
});

test("resolveWorkingDirectory stays inside the workspace", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sanad-action-inputs-"));
  fs.mkdirSync(path.join(root, ".git"));
  const child = path.join(root, "child");
  fs.mkdirSync(child);
  assert.equal(resolveWorkingDirectory(root, "child"), child);
  assert.throws(() => resolveWorkingDirectory(root, ".."), /inside GITHUB_WORKSPACE/);
});

test("resolveConfigPath rejects absolute and escaping paths", () => {
  assert.equal(resolveConfigPath("/workspace", ".sanad.toml"), ".sanad.toml");
  assert.throws(() => resolveConfigPath("/workspace", "../policy.toml"), /inside working-directory/);
  assert.throws(() => resolveConfigPath("/workspace", "/tmp/policy.toml"), /relative/);
});
