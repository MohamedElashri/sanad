"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { installSanad, releaseAsset, releaseTarget, sha256 } = require("../src/installer");

test("releaseTarget maps supported runner platforms", () => {
  assert.deepEqual(releaseTarget("linux", "x64"), {
    archive: "sanad_VERSION_Linux_x86_64.tar.gz",
    executable: "sanad",
    extension: "tar.gz",
  });
  assert.equal(releaseTarget("darwin", "arm64").archive, "sanad_VERSION_Darwin_arm64.tar.gz");
  assert.equal(releaseTarget("win32", "x64").archive, "sanad_VERSION_Windows_x86_64.zip");
  assert.throws(() => releaseTarget("freebsd", "x64"), /unsupported runner platform/);
  assert.throws(() => releaseTarget("linux", "riscv64"), /unsupported runner platform/);
});

test("releaseAsset requires an immutable release and GitHub digest", async () => {
  const hash = "a".repeat(64);
  const fetchRelease = async () => ({
    ok: true,
    status: 200,
    json: async () => ({
      tag_name: "v1.0.0",
      draft: false,
      prerelease: false,
      immutable: true,
      assets: [{
        name: "sanad_1.0.0_Linux_x86_64.tar.gz",
        digest: `sha256:${hash}`,
        browser_download_url: "https://github.com/MohamedElashri/sanad/releases/download/v1.0.0/sanad_1.0.0_Linux_x86_64.tar.gz",
      }],
    }),
  });
  assert.deepEqual(await releaseAsset("1.0.0", "sanad_1.0.0_Linux_x86_64.tar.gz", "", fetchRelease), {
    digest: hash,
    url: "https://github.com/MohamedElashri/sanad/releases/download/v1.0.0/sanad_1.0.0_Linux_x86_64.tar.gz",
  });

  const mutableRelease = async () => ({ ok: true, status: 200, json: async () => ({ tag_name: "v1.0.0", immutable: false, assets: [] }) });
  await assert.rejects(() => releaseAsset("1.0.0", "archive", "", mutableRelease), /not an immutable/);

  const wrongURL = async () => ({
    ok: true,
    status: 200,
    json: async () => ({
      tag_name: "v1.0.0",
      immutable: true,
      assets: [{ name: "archive", digest: `sha256:${hash}`, browser_download_url: "https://example.com/archive" }],
    }),
  });
  await assert.rejects(() => releaseAsset("1.0.0", "archive", "", wrongURL), /unexpected download URL/);
});

test("sha256 hashes downloaded bytes", async (context) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "sanad-action-hash-"));
  context.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const file = path.join(directory, "archive");
  fs.writeFileSync(file, "sanad\n");
  assert.equal(await sha256(file), "73b0147dda5102231ba4a576bc77bdea7d942131142fb583d3be31646bdbad27");
});

test("binary override requires explicit test mode", async () => {
  const oldBinary = process.env.SANAD_ACTION_BINARY;
  const oldTesting = process.env.SANAD_ACTION_TESTING;
  try {
    process.env.SANAD_ACTION_BINARY = __filename;
    delete process.env.SANAD_ACTION_TESTING;
    await assert.rejects(() => installSanad("1.0.0"), /restricted to the local test harness/);
    process.env.SANAD_ACTION_TESTING = "true";
    assert.equal(await installSanad("1.0.0"), fs.realpathSync(__filename));
  } finally {
    if (oldBinary === undefined) delete process.env.SANAD_ACTION_BINARY;
    else process.env.SANAD_ACTION_BINARY = oldBinary;
    if (oldTesting === undefined) delete process.env.SANAD_ACTION_TESTING;
    else process.env.SANAD_ACTION_TESTING = oldTesting;
  }
});
