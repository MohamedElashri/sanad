"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
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

async function installSanad(version, token = "") {
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
  const cached = tc.find("sanad", normalizedVersion, process.arch);
  if (cached) {
    const executable = path.join(cached, target.executable);
    if (!fs.existsSync(executable) || !fs.statSync(executable).isFile()) {
      throw new Error(`cached Sanad ${normalizedVersion} does not contain ${target.executable}`);
    }
    return executable;
  }

  const archiveName = target.archive.replace("VERSION", normalizedVersion);
  const asset = await releaseAsset(normalizedVersion, archiveName, token);
  const archivePath = await tc.downloadTool(asset.url);
  const actual = await sha256(archivePath);
  if (actual !== asset.digest) throw new Error(`GitHub release digest mismatch for ${archiveName}`);

  const extracted = target.extension === "zip" ? await tc.extractZip(archivePath) : await tc.extractTar(archivePath);
  const cachedDir = await tc.cacheDir(extracted, "sanad", normalizedVersion, process.arch);
  const executable = path.join(cachedDir, target.executable);
  if (!fs.existsSync(executable)) throw new Error(`release archive did not contain ${target.executable}`);
  if (process.platform !== "win32") fs.chmodSync(executable, 0o755);
  return executable;
}

module.exports = { installSanad, releaseAsset, releaseTarget, sha256 };
