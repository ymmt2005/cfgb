import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { serveBuiltSite } from "./site-server.mjs";

const rendererRoot = fileURLToPath(new URL("../..", import.meta.url));
export const code = 'const greeting = "こんにちは";\nconsole.log(greeting);';
export const diagram = "flowchart TD\nBrowser --> Search";
export const largeDiagram =
  "flowchart LR\n" +
  Array.from(
    { length: 12 },
    (_, i) => `Step${i}[Read diagram step ${i + 1}] --> Step${i + 1}`,
  ).join("\n");
const languages = {
  ja: { label: "日本語" },
  en: { label: "English" },
  "zh-Hans": { label: "简体中文" },
  ko: { label: "한국어" },
};

export async function startSite({
  basePath = "",
  static: staticSite = false,
  enabledLanguages = Object.keys(languages),
  presentation = false,
  siteImage = false,
  permissionsPolicy,
} = {}) {
  const work = buildSite(basePath, staticSite, enabledLanguages, presentation, siteImage);
  try {
    const site = await serveBuiltSite(path.join(work, "renderer", "dist"), {
      basePath,
      permissionsPolicy,
    });
    return {
      ...site,
      async close() {
        try {
          await site.close();
        } finally {
          rmSync(work, { recursive: true, force: true });
        }
      },
    };
  } catch (error) {
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
  const cspViolations = [];
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
  await page.exposeFunction("cfgbRecordTestCSP", (detail) => cspViolations.push(detail));
  await page.addInitScript(() => {
    window.cspViolations = [];
    document.addEventListener("securitypolicyviolation", (event) => {
      const detail = `${event.violatedDirective}: ${event.blockedURI}`;
      window.cspViolations.push(detail);
      window.cfgbRecordTestCSP(detail);
    });
  });
  try {
    await run(page, context);
    assert.deepEqual(failures, [], "browser runtime/resource errors");
    if (options.javaScriptEnabled !== false)
      assert.deepEqual(
        cspViolations,
        [],
        "generated CSP violations",
      );
  } finally {
    await context.close();
  }
}

function buildSite(basePath, staticSite, enabledLanguages, presentation, siteImage) {
  const configured = Object.fromEntries(
    enabledLanguages.map((locale) => [locale, languages[locale]]),
  );
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
      '```js title="example.js" {2}\n' + code + "\n```",
      "```text\n" + "0123456789 ".repeat(30) + "\n```",
      "```mermaid\n" + diagram + "\n```",
      "A reader note[^detail].",
      "[^detail]: A footnote reached through a native fragment link.",
      '<img id="responsive-density" alt="Density image" src="./assets/responsive-1.svg" srcset="./assets/responsive-1.svg 1x, ./assets/responsive-2.svg 2x" width="64" height="32">',
      '<picture><source media="(max-width: 600px)" srcset="./assets/responsive-2.svg 1x"><img id="responsive-picture" alt="Picture image" src="./assets/responsive-1.svg" width="64" height="32"></picture>',
      '<img id="responsive-width" alt="Width image" srcset="./assets/responsive-1.svg 64w, ./assets/responsive-2.svg 128w" sizes="64px" width="64" height="32">',
      '<img id="responsive-data" alt="Data image" srcset="data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20width=%2264%22%20height=%2232%22%3E%3C/svg%3E 1x, ./assets/responsive-2.svg 2x" width="64" height="32">',
    ].join("\n\n");
    const posts = enabledLanguages.map((locale) => ({
      id: `posts/2026/browser/${locale}`,
      file: path.join(content, "posts", "2026", "browser", `${locale}.md`),
      body:
        body +
        "\n\n" +
        ({
          "zh-Hans": "简体中文示例文章，支持语言切换。",
          ko: "한국어 예제 글에서 언어 전환을 확인합니다.",
        }[locale] || ""),
      group: "2026/browser",
      year: "2026",
      articleKey: "browser",
      locale,
      archive: { year: "2026", month: "10" },
      data: {
        title: {
          ja: "ブラウザの操作",
          en: "Browser interactions",
          "zh-Hans": "浏览器交互",
          ko: "브라우저 상호작용",
        }[locale],
        slug: "browser",
        publishedAt: "2026-09-30T16:30:00Z",
        topics: ["notes"],
        summary: "Interaction test article.",
        aliases: [`/${locale}/posts/old-browser/`],
      },
    }));
    for (const locale of ["ja", "ko"]) {
      if (!enabledLanguages.includes(locale)) continue;
      posts.push({
        id: `posts/2026/partial/${locale}`,
        file: path.join(content, "posts", "2026", "partial", `${locale}.md`),
        body: "## Partial translation\n\nAn article available in Japanese and Korean only.",
        group: "2026/partial",
        year: "2026",
        articleKey: "partial",
        locale,
        archive: { year: "2026", month: "10" },
        data: {
          title: "Partial translation",
          slug: `partial-${locale}`,
          publishedAt: "2026-09-30T16:30:00Z",
          topics: ["notes"],
          summary: "Partial translation group.",
        },
      });
    }
    if (enabledLanguages.includes("en"))
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
    if (enabledLanguages.includes("en"))
      posts.push({
        id: "posts/2026/large-diagram/en",
        file: path.join(content, "posts/2026/large-diagram/en.md"),
        body: `## Large diagram\n\n\`\`\`mermaid\n${largeDiagram}\n\`\`\``,
        group: "2026/large-diagram",
        year: "2026",
        articleKey: "large-diagram",
        locale: "en",
        archive: { year: "2026", month: "01" },
        data: {
          title: "Large diagram",
          slug: "large-diagram",
          publishedAt: "2026-01-01T00:00:00Z",
          topics: ["notes"],
          summary: "A wide diagram for expanded reading.",
        },
      });
    if (presentation)
      posts.push({
        id: "posts/2026/presentation/en",
        file: path.join(content, "posts/2026/presentation/en.md"),
        body: [
          "## Markdown presentation",
          "A **strong** word, an *emphasized* word, `inline code` and [an ordinary link](https://example.org/).",
          ...["NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"].map(
            (kind) => `> [!${kind}]\n> An alert with **Markdown** content.`,
          ),
          "> An ordinary blockquote.",
          "- First item\n- Second item\n\n1. First ordered item\n2. Second ordered item",
          "- [x] Completed task\n- [ ] Pending task",
          "| Message | Status |\n| --- | --- |\n| `" +
            "fully.qualified.ProtocolMessage.".repeat(5) +
            "` | Available |",
          "<details><summary>More details</summary><p>A native disclosure.</p></details>",
          '<img src="/media/2026/browser/responsive-1.svg" alt="Markdown presentation image" width="64" height="32">',
          "https://example.org/cached",
          "```mermaid\nflowchart LR\nDraft[Draft 日本語] -->|Review 中文| Review[Review 한국어] --> Publish[Publish]\n```",
          "```mermaid\nsequenceDiagram\nparticipant Author\nparticipant Reviewer\nAuthor->>Reviewer: Review this article\nNote over Author,Reviewer: Keep the labels readable\n```",
          "A note[^presentation].\n\n[^presentation]: A footnote with a backlink.",
        ].join("\n\n"),
        group: "2026/presentation",
        year: "2026",
        articleKey: "presentation",
        locale: "en",
        archive: { year: "2026", month: "01" },
        data: {
          title: "Markdown presentation",
          slug: "presentation",
          publishedAt: "2026-01-01T00:00:00Z",
          topics: ["notes"],
          summary: "Markdown appearance and scrolling.",
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
        topics: {
          notes: { ja: "メモ", en: "Notes", "zh-Hans": "笔记", ko: "메모" },
        },
      }),
    );
    mkdirSync(path.join(work, "linkcards"));
    if (presentation)
      writeFileSync(
        path.join(work, "linkcards", "presentation.json"),
        JSON.stringify({
          url: "https://example.org/cached",
          siteName: "Example",
          title: "Cached link card",
          description: "A local card fixture.",
        }),
      );
    const image = path.join(work, "site-image.svg");
    if (siteImage) writeFileSync(image, '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96"><rect width="96" height="96" fill="#8a3c18"/></svg>');
    const site = path.join(work, "site.json");
    writeFileSync(
      site,
      JSON.stringify({
        title: "CFGB Example",
        ...(siteImage ? { image } : {}),
        baseUrl: `https://example.invalid${basePath}`,
        static: staticSite,
        defaultLocale: enabledLanguages.includes("ja")
          ? "ja"
          : enabledLanguages[0],

        locales: configured,
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
