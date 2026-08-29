"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");

function fileHash(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

function managedFiles(workingDirectory, workflowPaths, configPath) {
  const files = new Set();
  const addPath = (relativePath, workflowDirectory = false) => {
    const absolute = path.resolve(workingDirectory, relativePath);
    const relative = path.relative(workingDirectory, absolute);
    if (relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
      throw new Error(`managed path escapes working-directory: ${relativePath}`);
    }
    if (!fs.existsSync(absolute)) return;
    const stat = fs.lstatSync(absolute);
    if (stat.isSymbolicLink()) throw new Error(`managed path must not be a symbolic link: ${relativePath}`);
    if (stat.isFile()) {
      if (!workflowDirectory || /\.ya?ml$/i.test(absolute)) files.add(absolute);
      return;
    }
    if (!stat.isDirectory()) return;
    for (const entry of fs.readdirSync(absolute, { withFileTypes: true })) {
      if (entry.isSymbolicLink()) continue;
      addPath(path.relative(workingDirectory, path.join(absolute, entry.name)), true);
    }
  };

  for (const workflowPath of workflowPaths) {
    const absolute = path.resolve(workingDirectory, workflowPath);
    const isDirectory = fs.existsSync(absolute) && fs.statSync(absolute).isDirectory();
    addPath(workflowPath, isDirectory);
  }
  addPath(".github/sanad.lock.json");
  addPath(configPath);
  return files;
}

function snapshotManagedFiles(workingDirectory, workflowPaths, configPath) {
  const snapshot = new Map();
  for (const file of managedFiles(workingDirectory, workflowPaths, configPath)) {
    snapshot.set(path.relative(workingDirectory, file).split(path.sep).join("/"), fileHash(file));
  }
  return snapshot;
}

function changedManagedFiles(before, after) {
  const changed = [];
  const names = new Set([...before.keys(), ...after.keys()]);
  for (const name of names) {
    if (before.get(name) !== after.get(name)) changed.push(name);
  }
  return changed.sort();
}

module.exports = { changedManagedFiles, managedFiles, snapshotManagedFiles };
