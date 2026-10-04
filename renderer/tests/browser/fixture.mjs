import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const rendererRoot = fileURLToPath(new URL("../..", import.meta.url));
export const code = 'const greeting = "こんにちは";\nconsole.log(greeting);';
export const diagram = "flowchart TD\nBrowser --> Search";

export async function startSite({ basePath = "", static: staticSite = false } = {}) {
  const work = buildSite(basePath, staticSite);
  let browser, server;
  try {
    const dist = path.join(work, "renderer", "dist");
    const headers = Object.fromEntries(
      readFileSync(path.join(dist, "_headers"), "utf8")
        .split("\n")
        .filter((line) => line.startsWith("  "))
        .map((line) => {
          const colon = line.indexOf(":");
          return [line.slice(0, colon).trim(), line.slice(colon + 1).trim()];
        }),
    );
    const types = {
      ".html": "text/html",
      ".js": "text/javascript",
      ".css": "text/css",
      ".svg": "image/svg+xml",
      ".wasm": "application/wasm",
      ".json": "application/json",
    };
    server = createServer((request, response) => {
      try {
        const pathname = decodeURIComponent(
          new URL(request.url, "http://localhost").pathname,
        );
        if (!pathname.startsWith(`${basePath}/`)) throw new Error("outside hosting prefix");
        const route = pathname.slice(basePath.length);
        const relative = route.endsWith("/") ? `${route}index.html` : route;
        const file = path.resolve(dist, `.${relative}`);
        if (!file.startsWith(`${dist}${path.sep}`))
          throw new Error("outside fixture");
        const body = readFileSync(file);
        response.writeHead(200, {
          ...headers,
          "Content-Type":
            types[path.extname(file)] || "application/octet-stream",
        });
        response.end(body);
      } catch {
        response.writeHead(404, headers);
        response.end();
      }
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    browser = await chromium.launch({
      executablePath: process.env.CFGB_TEST_CHROMIUM_EXECUTABLE,
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    return {
      browser,
      origin,
      async close() {
        await browser.close();
        await new Promise((resolve) => server.close(resolve));
        rmSync(work, { recursive: true, force: true });
      },
    };
  } catch (error) {
    await browser?.close();
    if (server?.listening)
      await new Promise((resolve) => server.close(resolve));
    rmSync(work, { recursive: true, force: true });
    throw error;
  }
}

// Use real browser APIs and fail on runtime, resource and CSP failures.
export async function withPage(site, options, run) {
  const context = await site.browser.newContext(options);
  const page = await context.newPage();
  page.setDefaultTimeout(10_000);
  const failures = [];
  page.on("pageerror", (error) => failures.push(error.message));
  page.on("response", (response) => {
    if (
      response.status() >= 400 &&
      ["script", "stylesheet", "fetch", "xhr", "image"].includes(
        response.request().resourceType(),
      )
    )
      failures.push(`${response.status()} ${response.url()}`);
  });
  await page.addInitScript(() => {
    window.cspViolations = [];
    document.addEventListener("securitypolicyviolation", (event) =>
      window.cspViolations.push(
        `${event.violatedDirective}: ${event.blockedURI}`,
      ),
    );
  });
  try {
    await run(page, context);
    assert.deepEqual(failures, [], "browser runtime/resource errors");
    if (options.javaScriptEnabled !== false)
      assert.deepEqual(
        await page.evaluate(() => window.cspViolations),
        [],
        "generated CSP violations",
      );
  } finally {
    await context.close();
  }
}

function buildSite(basePath, staticSite) {
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-browser-test-"));
  try {
    const renderer = path.join(work, "renderer");
    cpSync(rendererRoot, renderer, {
      recursive: true,
      filter(src) {
        const top = path.relative(rendererRoot, src).split(path.sep)[0];
        return !["node_modules", "dist", ".astro", "tests"].includes(top);
      },
    });
    symlinkSync(
      path.join(rendererRoot, "node_modules"),
      path.join(renderer, "node_modules"),
    );
    const content = path.join(work, "content");
    const padding = Array.from(
      { length: 20 },
      () => "A paragraph of ordinary article text for scrolling.",
    ).join("\n\n");
    const body = [
      "## Constructor",
      padding,
      "## `__proto__`",
      padding,
      "## Normal heading",
      padding,
      "Browserneedle is searchable in this article.",
      "```js\n" + code + "\n```",
      "```mermaid\n" + diagram + "\n```",
      "A reader note[^detail].",
      "[^detail]: A footnote reached through a native fragment link.",
      '<img id="responsive-density" alt="Density image" src="./assets/responsive-1.svg" srcset="./assets/responsive-1.svg 1x, ./assets/responsive-2.svg 2x" width="64" height="32">',
      '<picture><source media="(max-width: 600px)" srcset="./assets/responsive-2.svg 1x"><img id="responsive-picture" alt="Picture image" src="./assets/responsive-1.svg" width="64" height="32"></picture>',
      '<img id="responsive-width" alt="Width image" srcset="./assets/responsive-1.svg 64w, ./assets/responsive-2.svg 128w" sizes="64px" width="64" height="32">',
      '<img id="responsive-data" alt="Data image" srcset="data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20width=%2264%22%20height=%2232%22%3E%3C/svg%3E 1x, ./assets/responsive-2.svg 2x" width="64" height="32">',
    ].join("\n\n");
    const posts = ["ja", "en"].map((locale) => ({
      id: `posts/2026/browser/${locale}`,
      file: path.join(content, "posts", "2026", "browser", `${locale}.md`),
      body,
      group: "2026/browser",
      year: "2026",
      articleKey: "browser",
      locale,
      archive: { year: "2026", month: "10" },
      data: {
        title: locale === "ja" ? "ブラウザの操作" : "Browser interactions",
        slug: "browser",
        publishedAt: "2026-09-30T16:30:00Z",
        topics: ["notes"],
        summary: "Interaction test article.",
        aliases: [`/${locale}/posts/old-browser/`],
      },
    }));
    posts.push({
      id: "posts/2026/diagram-error/en",
      file: path.join(content, "posts", "2026", "diagram-error", "en.md"),
      body: "## Invalid diagram\n\n```mermaid\nthis is not a diagram\n```",
      group: "2026/diagram-error",
      year: "2026",
      articleKey: "diagram-error",
      locale: "en",
      archive: { year: "2026", month: "01" },
      data: {
        title: "Diagram syntax error",
        slug: "diagram-error",
        publishedAt: "2026-01-02T00:00:00Z",
        topics: ["notes"],
        summary: "A deliberately malformed diagram.",
      },
    });
    for (const post of posts) {
      mkdirSync(path.dirname(post.file), { recursive: true });
      writeFileSync(post.file, post.body);
    }
    for (const scale of [1, 2]) {
      const filename = `responsive-${scale}.svg`;
      const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${64 * scale}" height="${32 * scale}"><rect width="100%" height="100%" fill="blue"/></svg>`;
      for (const dir of [
        path.join(content, "posts/2026/browser/assets"),
        path.join(renderer, "public/media/2026/browser"),
      ]) {
        mkdirSync(dir, { recursive: true });
        writeFileSync(path.join(dir, filename), svg);
      }
    }
    const asideBody = [
      '<button popovertarget="browser-popover">Open aside popover</button>',
      '<div id="browser-popover" popover>Aside popover <button popovertarget="browser-popover" popovertargetaction="hide">Close aside popover</button></div>',
      '<button commandfor="browser-dialog" command="show-modal">Open aside dialog</button>',
      '<dialog id="browser-dialog">Aside dialog <button commandfor="browser-dialog" command="close">Close aside dialog</button></dialog>',
    ].join("\n");
    const asideFile = path.join(content, "aside", "en.md");
    mkdirSync(path.dirname(asideFile), { recursive: true });
    writeFileSync(asideFile, asideBody);
    const metadata = path.join(work, "metadata.json");
    writeFileSync(
      metadata,
      JSON.stringify({
        posts,
        prose: [
          {
            id: "aside/en",
            kind: "aside",
            locale: "en",
            file: asideFile,
            body: asideBody,
          },
        ],
        topics: { notes: { ja: "メモ", en: "Notes" } },
      }),
    );
    mkdirSync(path.join(work, "linkcards"));
    const site = path.join(work, "site.json");
    writeFileSync(
      site,
      JSON.stringify({
        title: "CFGB Example",
        baseUrl: `https://example.invalid${basePath}`,
        static: staticSite,
        defaultLocale: "ja",

        locales: { ja: { label: "日本語" }, en: { label: "English" } },
        contentRoot: content,
        topicsFile: path.join(work, "topics.yaml"),
        linkcardsDir: path.join(work, "linkcards"),
        metadataFile: metadata,
        latestPosts: 5,
      }),
    );
    execFileSync(
      process.execPath,
      [
        path.join(rendererRoot, "node_modules", "astro", "bin", "astro.mjs"),
        "build",
      ],
      {
        cwd: renderer,
        env: { ...process.env, CFGB_SITE_JSON: site },
        stdio: "pipe",
      },
    );
    execFileSync(
      process.execPath,
      [
        path.join(
          rendererRoot,
          "node_modules",
          "pagefind",
          "lib",
          "runner",
          "bin.cjs",
        ),
        "--site",
        "dist",
      ],
      { cwd: renderer, stdio: "pipe" },
    );
    return work;
  } catch (error) {
    rmSync(work, { recursive: true, force: true });
    throw error;
  }
}
