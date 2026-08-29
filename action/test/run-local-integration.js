"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");
const packageMetadata = require("../package.json");

const actionRoot = path.join(__dirname, "..");
const repositoryRoot = path.join(actionRoot, "..");
const bundle = path.join(actionRoot, "dist", "index.js");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: "utf8", ...options });
  if (result.error) throw result.error;
  return result;
}

function parseCommandFile(file) {
  const lines = fs.readFileSync(file, "utf8").split(/\r?\n/);
  const values = {};
  for (let index = 0; index < lines.length; index += 1) {
    const match = lines[index].match(/^([^<]+)<<(.+)$/);
    if (!match) continue;
    const [, name, delimiter] = match;
    const value = [];
    index += 1;
    while (index < lines.length && lines[index] !== delimiter) {
      value.push(lines[index]);
      index += 1;
    }
    values[name] = value.join("\n");
  }
  return values;
}

function actionEnvironment(testRoot, workspace, binary, mode, values = {}) {
  const files = {
    output: path.join(testRoot, `${mode}-${values.label || "run"}-output`),
    path: path.join(testRoot, `${mode}-${values.label || "run"}-path`),
    summary: path.join(testRoot, `${mode}-${values.label || "run"}-summary`),
  };
  for (const file of Object.values(files)) fs.writeFileSync(file, "");

  return {
    files,
    env: {
      ...process.env,
      GITHUB_OUTPUT: files.output,
      GITHUB_PATH: files.path,
      GITHUB_STEP_SUMMARY: files.summary,
      GITHUB_WORKSPACE: workspace,
      RUNNER_TEMP: testRoot,
      SANAD_ACTION_BINARY: binary,
      SANAD_ACTION_TESTING: "true",
      INPUT_MODE: mode,
      INPUT_CONFIG: ".sanad.toml",
      "INPUT_WORKING-DIRECTORY": ".",
      INPUT_FRESH: values.fresh ? "true" : "false",
      INPUT_STRICT: values.strict ? "true" : "false",
      INPUT_WRITE: values.write ? "true" : "false",
      INPUT_TOKEN: process.env.SANAD_ACTION_TEST_TOKEN || process.env.GITHUB_TOKEN || "",
    },
  };
}

function invokeAction(testRoot, workspace, binary, mode, values = {}) {
  const { env, files } = actionEnvironment(testRoot, workspace, binary, mode, values);
  const result = run(process.execPath, [bundle], { cwd: repositoryRoot, env });
  return { ...result, files, outputs: parseCommandFile(files.output) };
}

function requireSuccess(result, label) {
  assert.equal(result.status, 0, `${label} failed\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
  assert.equal(result.outputs.passed, "true", `${label} did not set passed=true`);
  assert.equal(result.outputs["sanad-version"], packageMetadata.version);
}

function main() {
  const testRoot = fs.mkdtempSync(path.join(os.tmpdir(), "sanad-action-local-"));
  const workspace = path.join(testRoot, "workspace");
  const binary = path.join(testRoot, process.platform === "win32" ? "sanad.exe" : "sanad");

  try {
    fs.mkdirSync(path.join(workspace, ".git"), { recursive: true });
    fs.mkdirSync(path.join(workspace, ".github", "workflows"), { recursive: true });
    fs.writeFileSync(path.join(workspace, ".sanad.toml"), 'cooldown = "0s"\n');
    fs.writeFileSync(
      path.join(workspace, ".github", "workflows", "fixture.yml"),
      "name: Action fixture\n\non: push\n\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n",
    );

    const ldflags = [
      `-X github.com/MohamedElashri/sanad/internal/cli.version=${packageMetadata.version}`,
      "-X github.com/MohamedElashri/sanad/internal/cli.commit=local-action-test",
      "-X github.com/MohamedElashri/sanad/internal/cli.date=local",
    ].join(" ");
    const build = run("go", ["build", "-ldflags", ldflags, "-o", binary, "./cmd/sanad"], {
      cwd: repositoryRoot,
      env: { ...process.env, GOCACHE: path.join(os.tmpdir(), "sanad-action-go-cache") },
    });
    assert.equal(build.status, 0, `Go build failed\n${build.stdout}\n${build.stderr}`);

    const invalid = invokeAction(testRoot, workspace, binary, "invalid");
    assert.equal(invalid.status, 1, "invalid mode should fail before installation");
    assert.equal(invalid.outputs.passed, "false");
    assert.equal(invalid.outputs.changed, "false");
    assert.equal(invalid.outputs["sanad-version"], packageMetadata.version);

    const setup = invokeAction(testRoot, workspace, binary, "setup");
    requireSuccess(setup, "setup mode");
    assert.match(fs.readFileSync(setup.files.path, "utf8"), new RegExp(path.dirname(binary).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));

    const failingCheck = invokeAction(testRoot, workspace, binary, "check", { label: "before" });
    assert.equal(failingCheck.status, 1, "check should reject the mutable fixture before apply");
    assert.equal(failingCheck.outputs.passed, "false");
    assert.equal(failingCheck.outputs.violations, "1");
    assert.ok(fs.existsSync(failingCheck.outputs["report-path"]));

    const plan = invokeAction(testRoot, workspace, binary, "plan");
    requireSuccess(plan, "plan mode");
    assert.equal(plan.outputs.changed, "false");
    assert.ok(Number(plan.outputs.updates) >= 1);
    assert.ok(fs.existsSync(plan.outputs["report-path"]));
    assert.ok(fs.existsSync(plan.outputs["pr-body-path"]));
    assert.equal(JSON.parse(fs.readFileSync(plan.outputs["report-path"], "utf8")).version, 1);

    const apply = invokeAction(testRoot, workspace, binary, "apply", { write: true });
    requireSuccess(apply, "apply mode");
    assert.equal(apply.outputs.changed, "true");
    const applyChanges = JSON.parse(apply.outputs["changed-files"]);
    assert.ok(applyChanges.includes(".github/workflows/fixture.yml"));
    assert.ok(applyChanges.includes(".github/sanad.lock.json"));
    assert.ok(fs.existsSync(apply.outputs["pr-body-path"]));

    const workflow = fs.readFileSync(path.join(workspace, ".github", "workflows", "fixture.yml"), "utf8");
    assert.match(workflow, /actions\/checkout@[0-9a-f]{40}/);
    assert.match(workflow, /# sanad: ref=v4/);

    const upgrade = invokeAction(testRoot, workspace, binary, "upgrade", { write: true });
    requireSuccess(upgrade, "upgrade mode");
    assert.equal(JSON.parse(fs.readFileSync(upgrade.outputs["report-path"], "utf8")).version, 2);

    const passingCheck = invokeAction(testRoot, workspace, binary, "check", { label: "after" });
    requireSuccess(passingCheck, "check mode after writes");
    assert.equal(passingCheck.outputs.violations, "0");

    process.stdout.write("Local action integration passed: invalid input, setup, failing check, plan, apply, upgrade, and passing check.\n");
  } finally {
    fs.rmSync(testRoot, { recursive: true, force: true });
  }
}

main();
