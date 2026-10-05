import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
  existsSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import {
  readVersion,
  sourceVersion,
  versionPath,
} from "../release-version.mjs";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

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

function detect(cwd, { status = "404", body = "{}", exit = "0" } = {}) {
  const bin = join(cwd, "test-bin");
  mkdirSync(bin);
  writeFileSync(
    join(bin, "curl"),
    `#!/usr/bin/env bash
set -euo pipefail
echo called >> "$CURL_TEST_CALLS"
while [[ "$#" -gt 0 ]]; do
  if [[ "$1" == --output ]]; then output="$2"; shift; fi
  shift
done
printf '%s' "$CURL_TEST_BODY" > "$output"
printf '%s' "$CURL_TEST_STATUS"
exit "$CURL_TEST_EXIT"
`,
    { mode: 0o755 },
  );
  const output = join(cwd, "github-output");
  const calls = join(cwd, "curl-calls");
  const result = spawnSync("bash", [join(root, "scripts/release-detect.sh")], {
    cwd,
    encoding: "utf8",
    env: {
      ...process.env,
      PATH: `${bin}:${process.env.PATH}`,
      GH_TOKEN: "test-token",
      GITHUB_OUTPUT: output,
      CURL_TEST_CALLS: calls,
      CURL_TEST_BODY: body,
      CURL_TEST_STATUS: status,
      CURL_TEST_EXIT: exit,
    },
  });
  if (result.error) throw result.error;
  return {
    ...result,
    called: existsSync(calls),
    outputs: existsSync(output)
      ? Object.fromEntries(
          readFileSync(output, "utf8")
            .trim()
            .split("\n")
            .map((line) => line.split("=")),
        )
      : {},
  };
}

test("unchanged version skips lookup and publishing", (t) => {
  const result = detect(repository(t, "v0.1.0", "v0.1.0"));
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.called, false);
  assert.deepEqual(result.outputs, {
    tag: "v0.1.0",
    release: "false",
    verify: "false",
  });
});

for (const tag of ["0.0.0-dev", "v1.2.3-alpha..1", "v1.2.3-01"]) {
  test(`development or GoReleaser-invalid tag ${tag} skips before lookup`, (t) => {
    const result = detect(repository(t, "v0.1.0", tag));
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.called, false);
    assert.equal(result.outputs.release, "false");
  });
}

// GoReleaser's NewVersion parser permits leading zeros in core versions.
for (const tag of ["v01.2.3", "v1.2.3-alpha.0", "v1.2.3-01a"]) {
  test(`GoReleaser-compatible tag ${tag} remains a release candidate`, (t) => {
    const result = detect(repository(t, "v0.1.0", tag));
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.outputs.tag, tag);
    assert.equal(result.outputs.release, "true");
  });
}

for (const previous of [null, "v0.1.0"]) {
  test(`new version releases with parent ${previous}`, (t) => {
    const result = detect(repository(t, previous, "v0.2.0"));
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(result.outputs, {
      tag: "v0.2.0",
      release: "true",
      verify: "true",
    });
  });
}

test("existing immutable release is verified without rebuilding", (t) => {
  const result = detect(repository(t, "v0.1.0", "v0.2.0"), {
    status: "200",
    body: JSON.stringify({ draft: false, immutable: true }),
  });
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(result.outputs, {
    tag: "v0.2.0",
    release: "false",
    verify: "true",
  });
});

test("existing draft can be retried", (t) => {
  const result = detect(repository(t, "v0.1.0", "v0.2.0"), {
    status: "200",
    body: JSON.stringify({ draft: true }),
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.outputs.release, "true");
});

test("existing mutable published release fails", (t) => {
  const result = detect(repository(t, "v0.1.0", "v0.2.0"), {
    status: "200",
    body: JSON.stringify({ draft: false, immutable: false }),
  });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /not immutable/);
});

for (const status of ["401", "403", "500"]) {
  test(`HTTP ${status} is a lookup error, not an absent release`, (t) => {
    const result = detect(repository(t, "v0.1.0", "v0.2.0"), { status });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, new RegExp(`HTTP ${status}`));
    assert.notEqual(result.outputs.release, "true");
  });
}

test("network and malformed response failures stop detection", (t) => {
  for (const response of [{ exit: "7" }, { status: "200", body: "not JSON" }]) {
    const result = detect(repository(t, "v0.1.0", "v0.2.0"), response);
    assert.notEqual(result.status, 0);
    assert.notEqual(result.outputs.release, "true");
  }
});

test("missing version and Git failures are not release candidates", (t) => {
  assert.throws(() => sourceVersion("package version\n"), /Missing Version/);
  let cwd = repository(t, "v0.1.0", "v0.2.0");
  rmSync(join(cwd, ".git"), { recursive: true });
  const result = detect(cwd);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Cannot inspect parent commit/);
  cwd = repository(t, null, "v0.2.0");
  rmSync(join(cwd, versionPath));
  assert.notEqual(detect(cwd).status, 0);
  assert.throws(() => readVersion(cwd), /ENOENT/);
});

test("release metadata uses the same Go version string as Bash detection", (t) => {
  const cwd = repository(t, null, "v0.2.0");
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
  assert.equal(metadata.cfgbVersion, detect(cwd).outputs.tag);
});
