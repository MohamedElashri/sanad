"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const tc = require("@actions/tool-cache");

function releaseTarget(platform = process.platform, arch = process.arch) {
  const osNames = { linux: "Linux", darwin: "Darwin", win32: "Windows" };
  const archNames = { x64: "x86_64", arm64: "arm64" };
  const osName = osNames[platform];
  const archName = archNames[arch];
  if (!osName || !archName) {
    throw new Error(`unsupported runner platform: ${platform}/${arch}`);
  }
  const extension = platform === "win32" ? "zip" : "tar.gz";
  return {
    archive: `sanad_VERSION_${osName}_${archName}.${extension}`,
    executable: platform === "win32" ? "sanad.exe" : "sanad",
    extension,
  };
}

function sha256(file) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(file);
    stream.on("error", reject);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolve(hash.digest("hex")));
  });
}

async function releaseAsset(version, archiveName, token = "", fetchImplementation = global.fetch) {
  const tag = `v${version}`;
  const headers = {
    accept: "application/vnd.github+json",
    "x-github-api-version": "2026-03-10",
    "user-agent": "sanad-action",
  };
  if (token) headers.authorization = `Bearer ${token}`;
  const response = await fetchImplementation(`https://api.github.com/repos/MohamedElashri/sanad/releases/tags/${tag}`, { headers });
  if (!response.ok) throw new Error(`GitHub release lookup failed with HTTP ${response.status}`);
  const release = await response.json();
  if (release.tag_name !== tag || release.draft || release.prerelease) throw new Error(`${tag} is not a published stable Sanad release`);
  if (release.immutable !== true) throw new Error(`${tag} is not an immutable GitHub release`);
  const asset = Array.isArray(release.assets) ? release.assets.find((item) => item.name === archiveName) : undefined;
  if (!asset) throw new Error(`${tag} does not contain ${archiveName}`);
  const digest = String(asset.digest || "").match(/^sha256:([a-fA-F0-9]{64})$/);
  if (!digest) throw new Error(`${archiveName} does not have a valid GitHub SHA-256 digest`);
  const expectedURL = `https://github.com/MohamedElashri/sanad/releases/download/${tag}/${archiveName}`;
  if (asset.browser_download_url !== expectedURL) {
    throw new Error(`${archiveName} has an unexpected download URL`);
  }
  return { digest: digest[1].toLowerCase(), url: asset.browser_download_url };
}

async function installSanad(version, token = "", deps = {}) {
  if (process.env.SANAD_ACTION_BINARY) {
    if (process.env.SANAD_ACTION_TESTING !== "true") {
      throw new Error("SANAD_ACTION_BINARY is restricted to the local test harness");
    }
    const override = fs.realpathSync(process.env.SANAD_ACTION_BINARY);
    if (!fs.statSync(override).isFile()) throw new Error("SANAD_ACTION_BINARY must name a file");
    return override;
  }
  const normalizedVersion = String(version).replace(/^v/, "");
  const target = releaseTarget();
  const cache = deps.toolCache || tc;
  const archiveName = target.archive.replace("VERSION", normalizedVersion);
  const asset = await releaseAsset(normalizedVersion, archiveName, token, deps.fetchImplementation);
  const cached = cache.find("sanad", normalizedVersion, process.arch);
  const storedArchive = `.sanad-release.${target.extension}`;
  let archivePath = cached ? path.join(cached, storedArchive) : "";
  const cacheHit = archivePath && fs.existsSync(archivePath) && fs.lstatSync(archivePath).isFile();
  let privateCopy = "";
  try {
    if (cacheHit) {
      privateCopy = fs.mkdtempSync(path.join(process.env.RUNNER_TEMP || os.tmpdir(), "sanad-archive-"));
      const copy = path.join(privateCopy, archiveName);
      fs.copyFileSync(archivePath, copy);
      archivePath = copy;
    } else {
      archivePath = await cache.downloadTool(asset.url);
    }
    const actual = await sha256(archivePath);
    if (actual !== asset.digest) throw new Error(`GitHub release digest mismatch for ${archiveName}`);

    // Execute a fresh extraction of verified bytes, never writable cached binaries.
    const extracted = target.extension === "zip" ? await cache.extractZip(archivePath) : await cache.extractTar(archivePath);
    const backend = process.platform === "win32" ? "gh-actions-lock.exe" : "gh-actions-lock";
    for (const name of [target.executable, backend]) {
      const file = path.join(extracted, name);
      if (!fs.existsSync(file) || !fs.lstatSync(file).isFile()) throw new Error(`release archive did not contain ${name}`);
      if (process.platform !== "win32") fs.chmodSync(file, 0o755);
    }
    if (!cacheHit) {
      fs.copyFileSync(archivePath, path.join(extracted, storedArchive));
      await cache.cacheDir(extracted, "sanad", normalizedVersion, process.arch);
    }
    return path.join(extracted, target.executable);
  } finally {
    if (privateCopy) fs.rmSync(privateCopy, { recursive: true, force: true });
  }
}

module.exports = { installSanad, releaseAsset, releaseTarget, sha256 };
