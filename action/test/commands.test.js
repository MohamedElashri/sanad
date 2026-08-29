"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const { commandArguments } = require("../src/index");

const base = { root: "/workspace", config: ".sanad.toml", fresh: false, strict: false, write: false };

test("commandArguments maps the supported modes without a shell", () => {
  const prefix = ["--root", "/workspace", "--config", ".sanad.toml", "--format", "json"];
  assert.deepEqual(commandArguments({ ...base, mode: "check", strict: true }, "/report", "/body"), [...prefix, "check", "--strict"]);
  assert.deepEqual(commandArguments({ ...base, mode: "plan" }, "/report", "/body"), [...prefix, "plan", "--out", "/report", "--pr-body-out", "/body"]);
  assert.deepEqual(commandArguments({ ...base, mode: "apply", write: true }, "/report", "/body"), [...prefix, "apply", "--pr-body-out", "/body", "--write", "--yes"]);
  assert.deepEqual(commandArguments({ ...base, mode: "upgrade", write: true }, "/report", "/body"), [...prefix, "upgrade", "--all", "--write", "--yes"]);
});
