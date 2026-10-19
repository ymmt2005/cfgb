import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";

const [wrangler, config, prefix = ""] = process.argv.slice(2);
const forceWorker = process.argv.includes("--worker-first");
const browserNavigation = process.argv.includes("--browser-navigation");
let runtimeConfig = config;
if (forceWorker) {
  // Exercise delegation even when Static Assets would handle a request without
  // invoking the Worker. This routing override belongs only to this test.
  const conf = JSON.parse(await readFile(config, "utf8"));
  conf.assets.run_worker_first = true;
  runtimeConfig = join(dirname(config), "wrangler-worker-first.json");
  await writeFile(runtimeConfig, JSON.stringify(conf), { mode: 0o600 });
}
const child = spawn(process.execPath, [wrangler, "dev", "--local", "--ip", "127.0.0.1", "--port", "0", "--config", runtimeConfig, "--no-bundle"], {
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
  for (const path of prefix ? ["/", ""] : ["/"]) {
    for (const method of ["GET", "HEAD"]) {
      const response = await request(path, { method, headers: { "Accept-Language": "en" } });
      assert.equal(response.status, 302, `${method} root ${prefix + path}: ${logs}`);
      assert.equal(response.headers.get("location"), prefix + "/en/");
      assert.equal(response.headers.get("cache-control"), "private, no-store");
      assert.match(response.headers.get("vary"), /Accept-Language/);
      if (method === "HEAD") assert.equal(await response.text(), "");
    }
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
  const missingPaths = ["/en/nonexistent-page/", "/en/nonexistent-image.png"];
  const conf = JSON.parse(await readFile(config, "utf8"));
  const expected404 = await readFile(join(conf.assets.directory, prefix.replace(/^\//, ""), "en", "404.html"), "utf8");
  // Node fetch derives Sec-Fetch-Mode from its mode, overwriting supplied
  // headers. Keep it for non-navigation requests only.
  for (const path of missingPaths) {
    const missing = await request(path, { mode: "cors" });
    assert.equal(missing.status, 404, `cors ${path}: missing assets must retain 404`);
    assert.equal(await missing.text(), expected404, `cors ${path}: nearest localized 404 page`);
  }
  if (browserNavigation) {
    const { chromium } = await import("../browser/node_modules/playwright/index.mjs");
    const { withPage } = await import("../browser/fixture.mjs");
    const browser = await chromium.launch({ executablePath: process.env.CFGB_TEST_CHROMIUM_EXECUTABLE || undefined });
    try {
      await withPage({ browser }, { locale: "en-US" }, async (page) => {
        const requestFailures = [];
        page.on("requestfailed", (request) => requestFailures.push(`${request.url()}: ${request.failure()?.errorText}`));
        for (const path of missingPaths) {
          const response = await page.goto(origin + prefix + path);
          assert(response, `navigation response for ${path}`);
          assert(response.request().isNavigationRequest(), `actual navigation for ${path}`);
          assert.equal(await response.request().headerValue("sec-fetch-mode"), "navigate");
          assert.equal(response.status(), 404, `navigate ${path}: missing assets must retain 404`);
          assert.equal(await response.text(), expected404, `navigate ${path}: nearest localized 404 page`);
        }
        assert.deepEqual(requestFailures, [], "navigation resource failures");
      });
    } finally {
      await browser.close();
    }
  }
  console.log(`Local Worker/Static Assets checks passed at ${prefix || "/"}; forced Worker: ${forceWorker}; browser navigation: ${browserNavigation ? "passed" : "not requested"}`);
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
