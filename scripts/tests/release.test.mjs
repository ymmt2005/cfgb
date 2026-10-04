import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { detectRelease, lookupRelease } from "../release-detect.mjs";
import {
  readVersion,
  sourceVersion,
  versionPath,
} from "../release-version.mjs";

function repository(t, previous, current) {
  const cwd = mkdtempSync(join(tmpdir(), "cfgb-release-test-"));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  const git = (...args) => execFileSync("git", args, { cwd, stdio: "pipe" });
  git("init", "--quiet");
  git("config", "user.name", "Release test");
  git("config", "user.email", "test@example.invalid");
  mkdirSync(dirname(join(cwd, versionPath)), { recursive: true });
  function commit(version) {
    writeFileSync(
      join(cwd, versionPath),
      `package version\nvar Version = "${version}"\n`,
    );
    git("add", ".");
    git("commit", "--quiet", "--allow-empty", "-m", version);
  }
  if (previous !== null) commit(previous);
  commit(current);
  return cwd;
}

test("unchanged version skips lookup and publishing", async (t) => {
  const cwd = repository(t, "v0.1.0", "v0.1.0");
  const result = await detectRelease(cwd, () =>
    assert.fail("unexpected API call"),
  );
  assert.deepEqual(result, { tag: "v0.1.0", release: false, verify: false });
});

test("development version skips publishing", async (t) => {
  const cwd = repository(t, "v0.1.0", "0.0.0-dev");
  const result = await detectRelease(cwd, () =>
    assert.fail("unexpected API call"),
  );
  assert.equal(result.release, false);
});

for (const previous of [null, "v0.1.0"]) {
  test(`new version releases with parent ${previous}`, async (t) => {
    const cwd = repository(t, previous, "v0.2.0");
    const result = await detectRelease(cwd, async (tag) => {
      assert.equal(tag, "v0.2.0");
      return null;
    });
    assert.deepEqual(result, { tag: "v0.2.0", release: true, verify: true });
  });
}

test("existing immutable release is verified without rebuilding", async (t) => {
  const cwd = repository(t, "v0.1.0", "v0.2.0");
  assert.deepEqual(
    await detectRelease(cwd, async () => ({ draft: false, immutable: true })),
    {
      tag: "v0.2.0",
      release: false,
      verify: true,
    },
  );
});

test("existing draft can be retried", async (t) => {
  const cwd = repository(t, "v0.1.0", "v0.2.0");
  assert.equal(
    (await detectRelease(cwd, async () => ({ draft: true }))).release,
    true,
  );
});

test("existing mutable published release fails", async (t) => {
  const cwd = repository(t, "v0.1.0", "v0.2.0");
  await assert.rejects(
    detectRelease(cwd, async () => ({ draft: false, immutable: false })),
    /not immutable/,
  );
});

test("release API distinguishes absence from lookup errors", async () => {
  assert.equal(
    await lookupRelease(
      "v0.2.0",
      async () => new Response(null, { status: 404 }),
    ),
    null,
  );
  for (const status of [401, 403, 500]) {
    await assert.rejects(
      lookupRelease("v0.2.0", async () => new Response(null, { status })),
      new RegExp(String(status)),
    );
  }
  await assert.rejects(
    lookupRelease("v0.2.0", async () => {
      throw new Error("offline");
    }),
    /offline/,
  );
});

test("missing or unreadable version and Git failures are not release candidates", async (t) => {
  assert.throws(() => sourceVersion("package version\n"), /Missing Version/);
  const cwd = repository(t, "v0.1.0", "v0.2.0");
  rmSync(join(cwd, ".git"), { recursive: true });
  await assert.rejects(detectRelease(cwd), /Cannot inspect parent commit/);
  rmSync(join(cwd, versionPath));
  assert.throws(() => readVersion(cwd), /ENOENT/);
});

test("release metadata uses the Go version string", (t) => {
  const cwd = repository(t, null, "v0.2.0");
  const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
  for (const path of [
    "renderer/package.json",
    "internal/build/build.go",
    "internal/worker/worker.go",
  ]) {
    mkdirSync(dirname(join(cwd, path)), { recursive: true });
    writeFileSync(join(cwd, path), readFileSync(join(root, path)));
  }
  execFileSync(process.execPath, [join(root, "scripts/release-metadata.mjs")], {
    cwd,
  });
  const metadata = JSON.parse(
    readFileSync(join(cwd, ".release/toolchain-requirements.json"), "utf8"),
  );
  assert.equal(metadata.cfgbVersion, "v0.2.0");
});
