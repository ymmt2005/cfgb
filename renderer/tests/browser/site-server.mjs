import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import path from "node:path";
import { chromium } from "playwright";

// Serve the actual generated headers and resources, including Pagefind's WASM.
export async function serveBuiltSite(dist, { basePath = "", permissionsPolicy } = {}) {
  const root = path.resolve(dist);
  const headers = Object.fromEntries(
    readFileSync(path.join(root, "_headers"), "utf8")
      .split("\n")
      .filter((line) => line.startsWith("  "))
      .map((line) => {
        const colon = line.indexOf(":");
        return [line.slice(0, colon).trim(), line.slice(colon + 1).trim()];
      }),
  );
  const types = {
    ".html": "text/html", ".js": "text/javascript", ".css": "text/css",
    ".svg": "image/svg+xml", ".png": "image/png", ".wasm": "application/wasm",
    ".json": "application/json", ".woff2": "font/woff2",
  };
  const server = createServer((request, response) => {
    try {
      const pathname = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
      if (!pathname.startsWith(`${basePath}/`)) throw new Error("outside hosting prefix");
      const route = pathname.slice(basePath.length);
      const relative = route.endsWith("/") ? `${route}index.html` : route;
      const file = path.resolve(root, `.${relative}`);
      if (!file.startsWith(`${root}${path.sep}`)) throw new Error("outside fixture");
      const body = readFileSync(file);
      response.writeHead(200, {
        ...headers,
        ...(permissionsPolicy ? { "Permissions-Policy": permissionsPolicy } : {}),
        "Content-Type": types[path.extname(file)] || "application/octet-stream",
      });
      response.end(body);
    } catch {
      response.writeHead(404, headers);
      response.end();
    }
  });
  let browser;
  async function close() {
    const failures = [];
    try { await browser?.close(); } catch (error) { failures.push(error); }
    if (server.listening) {
      try {
        await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      } catch (error) { failures.push(error); }
    }
    if (failures.length) throw new AggregateError(failures, "closing browser site");
  }
  try {
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    browser = await chromium.launch({ executablePath: process.env.CFGB_TEST_CHROMIUM_EXECUTABLE });
    return { browser, origin: `http://127.0.0.1:${server.address().port}`, close };
  } catch (error) {
    try { await close(); } catch (cleanup) { throw new AggregateError([error, cleanup], "starting browser site"); }
    throw error;
  }
}
