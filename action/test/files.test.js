"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { changedManagedFiles, snapshotManagedFiles } = require("../src/files");

test("managed snapshots report only content changes in configured files", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sanad-action-files-"));
  fs.mkdirSync(path.join(root, ".github", "workflows"), { recursive: true });
  const workflow = path.join(root, ".github", "workflows", "ci.yml");
  fs.writeFileSync(workflow, "name: CI\n");
  fs.writeFileSync(path.join(root, "unrelated.txt"), "before\n");
  const before = snapshotManagedFiles(root, [".github/workflows"], ".sanad.toml");

  fs.writeFileSync(workflow, "name: Changed\n");
  fs.writeFileSync(path.join(root, "unrelated.txt"), "after\n");
  fs.writeFileSync(path.join(root, ".github", "sanad.lock.json"), "{}\n");
  const after = snapshotManagedFiles(root, [".github/workflows"], ".sanad.toml");

  assert.deepEqual(changedManagedFiles(before, after), [".github/sanad.lock.json", ".github/workflows/ci.yml"]);
});
