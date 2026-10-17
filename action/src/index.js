"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const core = require("@actions/core");
const exec = require("@actions/exec");
const { installSanad } = require("./installer");
const { version } = require("../package.json");

const MODES = new Set(["check", "status", "plan", "sync", "update", "setup"]);

function boolean(value, name) {
  if (value !== "true" && value !== "false") throw new Error(`${name} must be true or false`);
  return value === "true";
}

function contained(root, requested) {
  const resolved = fs.realpathSync(path.resolve(root, requested));
  const relative = path.relative(root, resolved);
  if (relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) throw new Error("working-directory must stay inside GITHUB_WORKSPACE");
  if (!fs.statSync(resolved).isDirectory()) throw new Error("working-directory must be a directory");
  return resolved;
}

function inputs(get = core.getInput, env = process.env) {
  const mode = get("mode") || "check";
  if (!MODES.has(mode)) throw new Error(`mode must be one of ${[...MODES].join(", ")}`);
  const write = boolean(get("write") || "false", "write");
  if (write && mode !== "sync" && mode !== "update") throw new Error("write requires sync or update mode");
  const workspace = fs.realpathSync(env.GITHUB_WORKSPACE || process.cwd());
  const cwd = contained(workspace, get("working-directory") || ".");
  const config = get("config") || ".sanad.toml";
  if (path.isAbsolute(config) || config.split(/[\\/]/).includes("..")) throw new Error("config must stay inside working-directory");
  const base = get("base");
  if (base && mode !== "check") throw new Error("base requires check mode");
  const host = new URL(env.GITHUB_SERVER_URL || "https://github.com").hostname;
  return { mode, write, cwd, config, base, token: get("token") || "", repo: env.GITHUB_REPOSITORY ? `${host}/${env.GITHUB_REPOSITORY}` : "" };
}

function commandArguments(values) {
  const args = [values.mode, "--root", values.cwd, "--config", values.config, "--format", "json"];
  if (values.repo) args.push("--repo", values.repo);
  if (values.write) args.push("--write");
  if (values.base) args.push("--base", values.base);
  return args;
}

function parseReport(stdout, mode) {
  const r = JSON.parse(stdout);
  if (r.version !== 1 || r.command !== mode || typeof r.success !== "boolean" || typeof r.changed !== "boolean") throw new Error("unsupported Sanad report");
  for (const key of ["findings", "files", "decisions", "changes"]) if (!Array.isArray(r[key])) throw new Error(`invalid report ${key}`);
  for (const f of r.files) if (typeof f.path !== "string" || !f.path || path.isAbsolute(f.path) || f.path.includes("\\") || f.path.split("/").includes("..")) throw new Error("unsafe changed-file path");
  return r;
}

function markdown(r) {
  const safe = (s) => String(s || "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll("|", "&#124;").replace(/[\r\n]/g, " ");
  const lines = [`Sanad ${safe(r.command)}: ${r.success ? "passed" : "failed"}; verification ${safe(r.verification)}.`];
  for (const f of r.findings) lines.push(`\n- ${safe(f.severity)}: ${safe(f.message)}`);
  if (r.decisions.length) {
    lines.push("\n| Action | Target | Decision | Reason |", "| --- | --- | --- | --- |");
    for (const d of r.decisions) lines.push(`| ${safe(d.action)} | ${safe(d.to)} | ${safe(d.status)} | ${safe(d.reason)} |`);
  }
  if (r.changes.length) {
    lines.push("\n| Dependency | Before | After | Workflows |", "| --- | --- | --- | --- |");
    for (const c of r.changes) lines.push(`| ${safe(c.key)} | ${safe(c.before?.commit)} | ${safe(c.after?.commit)} | ${safe((c.after?.workflows || c.before?.workflows || []).join(", "))} |`);
  }
  return `${lines.join("\n")}\n`;
}

async function run(deps = {}) {
  const api = deps.core || core;
  const values = inputs(api.getInput);
  if (values.token) api.setSecret(values.token);
  const publicToken = !process.env.GITHUB_SERVER_URL || process.env.GITHUB_SERVER_URL === "https://github.com" ? values.token : "";
  const binary = await (deps.install || installSanad)(version, publicToken);
  const env = { ...process.env };
  if (values.token) { env.GH_TOKEN = values.token; env.GITHUB_TOKEN = values.token; }
  // A tenant's token must not be used to download the public Sanad release.
  const launch = deps.exec || exec.getExecOutput;
  const installed = await launch(binary, ["version", "--format", "json"], { cwd: values.cwd, env, silent: true });
  if (JSON.parse(installed.stdout).version !== version) throw new Error("installed Sanad version differs from the action version");
  api.addPath(path.dirname(binary));
  api.setOutput("sanad-version", version);
  if (values.mode === "setup") {
    await launch(binary, ["setup"], { cwd: values.cwd, env, silent: true });
    api.setOutput("passed", "true"); return;
  }
  const result = await launch(binary, commandArguments(values), { cwd: values.cwd, env, silent: true, ignoreReturnCode: true });
  let report;
  try { report = parseReport(result.stdout, values.mode); } catch (error) { throw new Error(`Sanad returned no valid report (exit ${result.exitCode}): ${error.message}`); }
  const dir = fs.mkdtempSync(path.join(process.env.RUNNER_TEMP || os.tmpdir(), "sanad-report-"));
  const reportPath = path.join(dir, "report.json");
  const bodyPath = path.join(dir, "review.md");
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  const body = markdown(report); fs.writeFileSync(bodyPath, body);
  api.setOutput("passed", String(result.exitCode === 0 && report.success));
  api.setOutput("changed", String(report.changed));
  api.setOutput("changed-files", JSON.stringify(report.changed ? report.files.map((f) => f.path) : []));
  api.setOutput("updates", String(report.decisions.filter((d) => d.status === "eligible").length));
  api.setOutput("pending", String(report.decisions.filter((d) => d.status === "pending").length));
  api.setOutput("report-path", reportPath); api.setOutput("pr-body-path", bodyPath);
  for (const f of report.findings.slice(0, 50)) {
    const properties = { title: `Sanad: ${f.code}` }; if (f.file) properties.file = f.file;
    if (f.severity === "error") api.error(f.message, properties); else if (f.severity === "warning") api.warning(f.message, properties);
  }
  await api.summary.addRaw(body).write();
  if (result.exitCode !== 0 || !report.success) api.setFailed(`Sanad ${values.mode} failed (exit ${result.exitCode}); see the report`);
}

if (require.main === module) run().catch((error) => core.setFailed(error.message));
module.exports = { inputs, commandArguments, parseReport, markdown, run };
