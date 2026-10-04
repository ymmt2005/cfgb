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
import { after, before, describe, test } from "node:test";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const rendererRoot = fileURLToPath(new URL("../..", import.meta.url));
let browser, server, work, origin;

describe("generated site interactions", { timeout: 180_000 }, () => {
  before(async () => {
    work = buildSite();
    const dist = path.join(work, "renderer", "dist");
    // Exercise the deployed CSP as well as the actual generated HTML/scripts.
    const csp = readFileSync(path.join(dist, "_headers"), "utf8").match(
      /Content-Security-Policy: (.+)/,
    )[1];
    server = createServer((request, response) => {
      try {
        const pathname = decodeURIComponent(
          new URL(request.url, "http://localhost").pathname,
        );
        const relative = pathname.endsWith("/")
          ? `${pathname}index.html`
          : pathname;
        const file = path.resolve(dist, `.${relative}`);
        if (!file.startsWith(`${dist}${path.sep}`))
          throw new Error("outside fixture");
        const types = {
          ".html": "text/html",
          ".js": "text/javascript",
          ".css": "text/css",
          ".svg": "image/svg+xml",
        };
        response.writeHead(200, {
          "Content-Type":
            types[path.extname(file)] || "application/octet-stream",
          "Content-Security-Policy": csp,
        });
        response.end(readFileSync(file));
      } catch {
        response.writeHead(404);
        response.end();
      }
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({
      executablePath: process.env.CFGB_TEST_CHROMIUM_EXECUTABLE,
    });
  });
  after(async () => {
    await browser?.close();
    if (server?.listening)
      await new Promise((resolve) => server.close(resolve));
    if (work) rmSync(work, { recursive: true, force: true });
  });

  test("unknown persisted themes fall back to system", async () => {
    for (const value of ["constructor", "__proto__", "toString", "unknown"]) {
      const page = await browser.newPage({ colorScheme: "dark" });
      try {
        await page.addInitScript(
          (saved) => localStorage.setItem("cfgb-theme", saved),
          value,
        );
        await page.goto(`${origin}/en/posts/browser/`);
        await page.waitForFunction(() =>
          document.documentElement.classList.contains("js"),
        );
        assert.equal(
          await page.locator("html").getAttribute("data-theme"),
          null,
        );
        await page.getByRole("button", { name: "Theme", exact: true }).click();
        assert.equal(
          await page
            .getByRole("option", { name: "Match system" })
            .getAttribute("aria-selected"),
          "true",
        );
      } finally {
        await page.close();
      }
    }
  });

  test("named theme listbox supports keyboard choice, persistence, and escape", async () => {
    const page = await browser.newPage();
    try {
      await page.goto(`${origin}/en/posts/browser/`);
      const button = page.getByRole("button", { name: "Theme", exact: true });
      await button.focus();
      await page.keyboard.press("ArrowDown");
      assert.equal(
        await page
          .getByRole("listbox", { name: "Theme", exact: true })
          .isVisible(),
        true,
      );
      assert.equal(await button.getAttribute("aria-expanded"), "true");
      assert.equal(
        await page
          .getByRole("option", { name: "Match system" })
          .evaluate((el) => el === document.activeElement),
        true,
      );
      await page.keyboard.press("End");
      await page.keyboard.press("Enter");
      assert.equal(
        await page.locator("html").getAttribute("data-theme"),
        "dark",
      );
      assert.equal(await button.getAttribute("aria-expanded"), "false");
      assert.equal(
        await button.evaluate((el) => el === document.activeElement),
        true,
      );
      await page.reload();
      assert.equal(
        await page.locator("html").getAttribute("data-theme"),
        "dark",
      );
      await button.click();
      await page.keyboard.press("Home");
      await page.keyboard.press("ArrowDown");
      await page.keyboard.press("Space");
      assert.equal(
        await page.locator("html").getAttribute("data-theme"),
        "light",
      );
      await button.click();
      await page.keyboard.press("ArrowUp");
      await page.keyboard.press("Escape");
      assert.equal(
        await page.locator("html").getAttribute("data-theme"),
        "light",
      );
      assert.equal(await button.getAttribute("aria-expanded"), "false");
      assert.equal(
        await button.evaluate((el) => el === document.activeElement),
        true,
      );
      await button.click();
      await page.keyboard.press("Home");
      await page.keyboard.press("Enter");
      assert.equal(await page.locator("html").getAttribute("data-theme"), null);
      await page.reload();
      assert.equal(await page.locator("html").getAttribute("data-theme"), null);
    } finally {
      await page.close();
    }
  });

  test("theme controls still work when storage is unavailable", async () => {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.addInitScript(() => {
        Object.defineProperty(window, "localStorage", {
          get() {
            throw new Error("storage unavailable");
          },
        });
      });
      await page.goto(`${origin}/ja/posts/browser/`);
      await page.getByRole("button", { name: "テーマ", exact: true }).click();
      assert.equal(
        await page
          .getByRole("listbox", { name: "テーマ", exact: true })
          .isVisible(),
        true,
      );
      await page.locator('[role="option"][data-value="dark"]').click();
      assert.equal(
        await page.locator("html").getAttribute("data-theme"),
        "dark",
      );
      assert.deepEqual(errors, []);
    } finally {
      await page.close();
    }
  });

  test("native scroll observation updates both TOCs for prototype-like heading IDs", async () => {
    for (const width of [1440, 375]) {
      const page = await browser.newPage({ viewport: { width, height: 900 } });
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      try {
        await page.goto(`${origin}/en/posts/browser/`);
        for (const id of ["constructor", "__proto__", "normal-heading"]) {
          await page
            .locator(`.prose [id="${id}"]`)
            .evaluate((el) =>
              el.scrollIntoView({ block: "start", behavior: "instant" }),
            );
          await page.waitForFunction((id) => {
            const links = [
              ...document.querySelectorAll(".toc a, .toc-mobile a"),
            ];
            const matching = links.filter(
              (link) => decodeURIComponent(link.hash.slice(1)) === id,
            );
            return (
              matching.length === 2 &&
              matching.every(
                (link) => link.getAttribute("aria-current") === "true",
              ) &&
              links.filter(
                (link) => link.getAttribute("aria-current") === "true",
              ).length === 2
            );
          }, id);
        }
        assert.deepEqual(errors, []);
      } finally {
        await page.close();
      }
    }
  });
});

function buildSite() {
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
    ].join("\n\n");
    const posts = ["ja", "en"].map((locale) => ({
      id: `posts/2026/browser/${locale}`,
      file: path.join(content, "posts", "2026", "browser", `${locale}.md`),
      body,
      group: "2026/browser",
      year: "2026",
      articleKey: "browser",
      locale,
      data: {
        title: "Browser interactions",
        slug: "browser",
        publishedAt: "2026-01-02T00:00:00Z",
        topics: ["notes"],
        summary: "Interaction test article.",
      },
    }));
    for (const post of posts) {
      mkdirSync(path.dirname(post.file), { recursive: true });
      writeFileSync(post.file, post.body);
    }
    const metadata = path.join(work, "metadata.json");
    writeFileSync(
      metadata,
      JSON.stringify({
        posts,
        prose: [],
        topics: { notes: { ja: "メモ", en: "Notes" } },
      }),
    );
    mkdirSync(path.join(work, "linkcards"));
    const site = path.join(work, "site.json");
    writeFileSync(
      site,
      JSON.stringify({
        title: "CFGB Example",
        baseUrl: "https://example.invalid",
        defaultLocale: "ja",
        timezone: "UTC",
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
    return work;
  } catch (error) {
    rmSync(work, { recursive: true, force: true });
    throw error;
  }
}
