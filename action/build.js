"use strict";

const fs = require("node:fs");
const path = require("node:path");
const { buildSync } = require("esbuild");
const lock = require("./package-lock.json");

const result = buildSync({
  absWorkingDir: __dirname,
  entryPoints: ["src/index.js"],
  outfile: "dist/index.js",
  bundle: true,
  platform: "node",
  target: "node24",
  format: "cjs",
  minify: true,
  // Escape multiline strings so generated code has no trailing whitespace.
  supported: { "template-literal": false },
  legalComments: "eof",
  metafile: true,
});

// Preserve full license texts for the dependencies actually included in the bundle.
const inputs = Object.keys(result.metafile.inputs);
const licenses = [];
for (const name of Object.keys(lock.packages).sort()) {
  if (!name || !inputs.some((input) => input.startsWith(`${name}/`))) continue;
  const directory = path.join(__dirname, name);
  const pkg = JSON.parse(fs.readFileSync(path.join(directory, "package.json"), "utf8"));
  const files = fs.readdirSync(directory).filter((file) => /^licen[sc]e(?:\.(?:md|txt))?$/i.test(file)).sort();
  if (!files.length) throw new Error(`No license file found for bundled dependency ${pkg.name}`);
  licenses.push(`${pkg.name}\n${pkg.license || "See license below"}\n${files.map((file) => fs.readFileSync(path.join(directory, file), "utf8").trim()).join("\n\n")}\n`);
}
fs.writeFileSync(path.join(__dirname, "dist/licenses.txt"), licenses.join("\n"));
