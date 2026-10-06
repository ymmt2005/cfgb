import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

const [wrangler, config, prefix = ""] = process.argv.slice(2);
const child = spawn(process.execPath, [wrangler, "dev", "--local", "--ip", "127.0.0.1", "--port", "0", "--config", config, "--no-bundle"], {
  stdio: ["ignore", "pipe", "pipe"],
  env: { ...process.env, WRANGLER_SEND_METRICS: "false", CLOUDFLARE_CF_FETCH_ENABLED: "false", CI: "true", NO_COLOR: "1" },
});
let logs = "";
const closed = new Promise((resolve, reject) => {
  child.once("error", reject);
  child.once("close", (code, signal) => resolve({ code, signal }));
});

try {
  const origin = await new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error(`Wrangler readiness timed out:\n${logs}`)), 30000);
    const read = (chunk) => {
      logs += chunk.toString();
      const ready = logs.match(/Ready on (http:\/\/127\.0\.0\.1:\d+)/);
      if (ready) {
        clearTimeout(timeout);
        resolve(ready[1]);
      }
    };
    child.stdout.on("data", read);
    child.stderr.on("data", read);
    closed.then(({ code, signal }) => {
      clearTimeout(timeout);
      reject(new Error(`Wrangler exited before readiness (${code}, ${signal}):\n${logs}`));
    }, (error) => { clearTimeout(timeout); reject(error); });
  });
  const request = (path, options = {}) => fetch(origin + prefix + path, {
    redirect: "manual", signal: AbortSignal.timeout(10000), ...options,
  });
  for (const method of ["GET", "HEAD"]) {
    const response = await request("/", { method, headers: { "Accept-Language": "en" } });
    assert.equal(response.status, 302, `${method} root: ${logs}`);
    assert.equal(response.headers.get("location"), prefix + "/en/");
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.match(response.headers.get("vary"), /Accept-Language/);
    if (method === "HEAD") assert.equal(await response.text(), "");
  }
  const locale = await request("/__locale?lang=ja&next=" + encodeURIComponent(prefix + "/ja/"));
  assert.equal(locale.status, 303);
  assert.equal(locale.headers.get("location"), prefix + "/ja/");
  assert.match(locale.headers.get("set-cookie"), /cfgb_locale=ja/);
  const remembered = await request("/", { headers: { Cookie: "cfgb_locale=ja", "Accept-Language": "en" } });
  assert.equal(remembered.headers.get("location"), prefix + "/ja/");
  const page = await request("/en/");
  assert.equal(page.status, 200);
  assert.match(page.headers.get("content-type"), /text\/html/);
  assert.equal(page.headers.get("x-content-type-options"), "nosniff");
  assert.match(await page.text(), /operator edit before upload/);
  const search = await request("/pagefind/pagefind.js");
  assert.equal(search.status, 200);
  assert.match(await search.text(), /pagefind/i);
  for (const path of ["/en/nonexistent-page/", "/en/nonexistent-image.png"]) {
    const missing = await request(path, { headers: { "Sec-Fetch-Mode": "navigate" } });
    assert.equal(missing.status, 404, `${path}: missing assets must retain 404`);
    if (path.endsWith("/")) {
      const conf = JSON.parse(await readFile(config, "utf8"));
      const expected = await readFile(join(conf.assets.directory, prefix.replace(/^\//, ""), "en", "404.html"), "utf8");
      assert.equal(await missing.text(), expected, "nearest localized 404 page");
    }
  }
  console.log(`Local Worker/Static Assets checks passed at ${prefix || "/"}`);
} finally {
  const stopping = child.exitCode === null && child.signalCode === null;
  if (stopping) {
    if (!child.kill("SIGTERM")) throw new Error("Could not stop local Wrangler");
  }
  let timeout;
  const shutdownTimeout = new Promise((resolve, reject) => {
    timeout = setTimeout(() => {
      const killed = child.kill("SIGKILL");
      reject(new Error(`Local Wrangler shutdown timed out; forced stop: ${killed}`));
    }, 10000);
  });
  const { code, signal } = await Promise.race([closed, shutdownTimeout]).finally(() => clearTimeout(timeout));
  // Wrangler's launcher can propagate SIGTERM as exit 128 + 15 rather than
  // a signal. Accept that result only when this helper requested shutdown.
  if (code !== 0 && !(stopping && (signal === "SIGTERM" || code === 143))) {
    throw new Error(`Local Wrangler failed (${code}, ${signal}):\n${logs}`);
  }
}
