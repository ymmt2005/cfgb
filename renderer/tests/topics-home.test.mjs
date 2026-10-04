import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const rendererRoot = fileURLToPath(new URL("..", import.meta.url));

test(
  "generated topic pages use catalog entries and home preserves zero latest posts",
  { timeout: 180_000 },
  () => {
    const work = mkdtempSync(path.join(tmpdir(), "cfgb-topics-home-"));
    try {
      const renderer = path.join(work, "renderer");
      cpSync(rendererRoot, renderer, {
        recursive: true,
        filter(src) {
          const top = path.relative(rendererRoot, src).split(path.sep)[0];
          return !["node_modules", "dist", ".astro"].includes(top);
        },
      });
      symlinkSync(
        path.join(rendererRoot, "node_modules"),
        path.join(renderer, "node_modules"),
      );
      const content = path.join(work, "content");
      const file = path.join(content, "posts", "2026", "guide", "en.md");
      mkdirSync(path.dirname(file), { recursive: true });
      writeFileSync(file, "Article body.\n");
      const metadataFile = path.join(work, "metadata.json");
      writeFileSync(
        metadataFile,
        JSON.stringify({
          topics: Object.fromEntries([
            ["constructor", { en: "Registered constructor" }],
            ["__proto__", { en: "Registered prototype" }],
          ]),
          posts: [
            {
              id: "posts/2026/guide/en",
              file,
              body: "Article body.\n",
              group: "2026/guide",
              year: "2026",
              articleKey: "guide",
              locale: "en",
              archive: { year: "2026", month: "09" },
              data: {
                title: "Visible article",
                slug: "guide",
                publishedAt: "2026-09-20T00:00:00Z",
                topics: [
                  "constructor",
                  "__proto__",
                  "toString",
                  "hasOwnProperty",
                ],
              },
            },
          ],
          prose: [],
        }),
      );
      const siteFile = path.join(work, "site.json");
      writeFileSync(
        siteFile,
        JSON.stringify({
          title: "Example",
          baseUrl: "https://example.invalid",
          defaultLocale: "en",
          locales: { en: { label: "English" } },
          contentRoot: content,
          metadataFile,
          latestPosts: 0,
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
          env: { ...process.env, CFGB_SITE_JSON: siteFile },
          stdio: "pipe",
        },
      );
      const dist = path.join(renderer, "dist");
      for (const [name, label] of [
        ["constructor", "Registered constructor"],
        ["__proto__", "Registered prototype"],
      ]) {
        const html = readFileSync(
          path.join(dist, "en", "topics", name, "index.html"),
          "utf8",
        );
        assert.ok(html.includes(label));
        assert.ok(html.includes("Visible article"));
      }
      for (const name of ["toString", "hasOwnProperty"]) {
        assert.equal(
          existsSync(path.join(dist, "en", "topics", name)),
          false,
          name,
        );
      }
      const home = readFileSync(path.join(dist, "en", "index.html"), "utf8");
      assert.equal(home.includes("Visible article"), false);
      assert.ok(
        readFileSync(
          path.join(dist, "en", "posts", "index.html"),
          "utf8",
        ).includes("Visible article"),
      );
      assert.ok(
        readFileSync(path.join(dist, "sitemap-0.xml"), "utf8").includes(
          "/en/posts/guide/",
        ),
      );
    } finally {
      rmSync(work, { recursive: true, force: true });
    }
  },
);
