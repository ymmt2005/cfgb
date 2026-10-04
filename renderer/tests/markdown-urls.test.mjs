import assert from "node:assert/strict";
import test from "node:test";
import { execFileSync } from "node:child_process";
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  writeFileSync,
  readFileSync,
  symlinkSync,
  existsSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "parse5";
import { createIndex } from "pagefind";
import { gunzipSync } from "node:zlib";

const root = fileURLToPath(new URL("..", import.meta.url));

test(
  "Markdown image and link patterns resolve in the actual built output",
  { timeout: 180_000 },
  async () => {
    const work = mkdtempSync(path.join(tmpdir(), "cfgb-pattern-matrix-"));
    try {
      const renderer = path.join(work, "renderer");
      cpSync(root, renderer, {
        recursive: true,
        filter: (src) =>
          !["node_modules", ".astro", "dist"].includes(
            path.relative(root, src).split(path.sep)[0],
          ),
      });
      symlinkSync(
        path.join(root, "node_modules"),
        path.join(renderer, "node_modules"),
      );
      const content = path.join(work, "content");
      const svg =
        '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="32" viewBox="0 0 64 32"><view id="detail" viewBox="0 0 32 32"/><rect width="64" height="32" fill="blue"/></svg>';
      const png = Buffer.from(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
        "base64",
      );
      function write(file, body) {
        mkdirSync(path.dirname(file), { recursive: true });
        writeFileSync(file, body);
      }
      const cases = [];
      function add(name, body, checks) {
        cases.push({ name, body, checks });
      }
      const image = (alt, width = "64", height = "32", suffix = "") => ({
        kind: "processed-image",
        alt,
        width,
        height,
        suffix,
      });
      const link = (text, suffix = "") => ({ kind: "link", text, suffix });
      const raw = (alt, suffix = "") => ({ kind: "raw-image", alt, suffix });
      add("inline-svg", '![Inline](./assets/picture.svg "Image title")', [
        { ...image("Inline"), title: "Image title" },
      ]);
      add("inline-raster", "![Raster](./assets/pixel.png)", [
        image("Raster", "1", "1"),
      ]);
      add(
        "reference-full",
        '![Full][pic]\n\n[pic]: ./assets/picture.svg "Reference title"',
        [{ ...image("Full"), title: "Reference title" }],
      );
      add(
        "reference-collapsed",
        "![Collapsed][]\n\n[Collapsed]: ./assets/picture.svg",
        [image("Collapsed")],
      );
      add(
        "reference-shortcut",
        "![Shortcut]\n\n[Shortcut]: ./assets/picture.svg",
        [image("Shortcut")],
      );
      add(
        "reference-normalized",
        "![Normalized][  pIc  ]\n\n[PIC]: ./assets/picture.svg",
        [image("Normalized")],
      );
      add(
        "reference-unicode-case",
        "![Fold][STRASSE]\n\n[straße]: ./assets/picture.svg",
        [image("Fold")],
      );
      add(
        "reference-many-images",
        "![First][pic]\n\n![Second][pic]\n\n[pic]: ./assets/picture.svg",
        [image("First"), image("Second")],
      );
      add(
        "reference-image-and-link",
        '![Shared][pic]\n\n[Download][pic]\n\n[pic]: ./assets/picture.svg "Shared title"',
        [
          { ...image("Shared"), title: "Shared title" },
          { ...link("Download"), title: "Shared title" },
        ],
      );
      add(
        "reference-collapsed-mixed",
        "![Shared][]\n\n[Shared][]\n\n[Shared]: ./assets/picture.svg",
        [image("Shared"), link("Shared")],
      );
      add(
        "reference-shortcut-mixed",
        "![Shared]\n\n[Shared]\n\n[Shared]: ./assets/picture.svg",
        [image("Shared"), link("Shared")],
      );
      add(
        "reference-link-only",
        "[Download][pic]\n\n[pic]: ./assets/picture.svg",
        [link("Download")],
      );
      add(
        "mixed-normalized-label",
        "![Normalized][  pIc  ]\n\n[Download][PIC]\n\n[PIC]: ./assets/picture.svg",
        [image("Normalized"), link("Download")],
      );
      add(
        "first-definition-wins",
        "![First][pic]\n\n[pic]: ./assets/picture.svg\n[pic]: ./assets/missing.svg",
        [image("First")],
      );
      add("image-in-alert", "> [!NOTE]\n> ![Alert](./assets/picture.svg)", [
        image("Alert"),
      ]);
      add(
        "image-in-table",
        "| Image |\n| --- |\n| ![Table](./assets/picture.svg) |",
        [image("Table")],
      );
      add(
        "image-in-footnote",
        "See [^note].\n\n[^note]: ![Footnote](./assets/picture.svg)",
        [image("Footnote")],
      );
      for (const [label, url] of [
        ["space", "a%20b.svg"],
        ["unicode", "%E5%9B%B3.svg"],
        ["nested", "nested/picture.svg"],
      ]) {
        add("inline-" + label, `![Path](./assets/${url})`, [image("Path")]);
        add("reference-" + label, `![Path][pic]\n\n[pic]: ./assets/${url}`, [
          image("Path"),
        ]);
        add("link-" + label, `[Download](./assets/${url})`, [link("Download")]);
      }
      add("inline-space-angle", "![Space](<./assets/a b.svg>)", [
        image("Space"),
      ]);
      add("inline-unicode-literal", "![Unicode](./assets/図.svg)", [
        image("Unicode"),
      ]);
      add("raw-double", '<img src="./assets/a%20b.svg" alt="Raw">', [
        raw("Raw"),
      ]);
      add("raw-single", "<img src='./assets/picture.svg' alt='Raw'>", [
        raw("Raw"),
      ]);
      add(
        "raw-density",
        '<img src="./assets/picture.svg" srcset="./assets/picture.svg 1x, ./assets/a%20b.svg 2x" alt="Responsive">',
        [
          {
            kind: "srcset",
            tag: "img",
            candidates: [
              ["picture.svg", "1x"],
              ["a%20b.svg", "2x"],
            ],
          },
        ],
      );
      add(
        "raw-width",
        "<picture><source srcset='./assets/picture.svg 320w, ./assets/a%20b.svg 640w' sizes='100vw'><img src='./assets/picture.svg' alt='Responsive'></picture>",
        [
          {
            kind: "srcset",
            tag: "source",
            candidates: [
              ["picture.svg", "320w"],
              ["a%20b.svg", "640w"],
            ],
          },
        ],
      );
      add(
        "raw-encoded-srcset",
        '<img srcset="./assets/a%23b.svg 1x, ./assets/a%26b.svg 2x" alt="Encoded candidates">',
        [
          {
            kind: "srcset",
            tag: "img",
            candidates: [
              ["a%23b.svg", "1x"],
              ["a%26b.svg", "2x"],
            ],
          },
        ],
      );
      add(
        "raw-unquoted-html-block",
        "<div><img src=./assets/picture.svg alt=Raw></div>",
        [raw("Raw")],
      );
      add("raw-link", '<a href="./assets/a%20b.svg">Download</a>', [
        link("Download"),
      ]);
      add(
        "external",
        "![Remote](https://example.invalid/picture.png)\n\n![Protocol](//example.invalid/picture.png)\n\n[External](https://example.invalid/page?x=1#detail)",
        [
          {
            kind: "external-image",
            alt: "Remote",
            url: "https://example.invalid/picture.png",
          },
          {
            kind: "external-image",
            alt: "Protocol",
            url: "//example.invalid/picture.png",
          },
          {
            kind: "external-link",
            text: "External",
            url: "https://example.invalid/page?x=1#detail",
          },
        ],
      );
      add("root-relative", "![Public](/media/home/picture.svg)", [
        {
          kind: "external-image",
          alt: "Public",
          url: "/media/home/picture.svg",
        },
      ]);
      add(
        "linked-image",
        "[![Thumbnail](./assets/picture.svg)](./assets/picture.svg)",
        [image("Thumbnail"), { kind: "image-link", suffix: "" }],
      );
      add(
        "article-link-inline",
        "[Article](../inline-svg/en.md?view=1#linked-heading)",
        [
          {
            kind: "article-link",
            text: "Article",
            suffix: "?view=1#linked-heading",
          },
        ],
      );
      add(
        "article-link-reference",
        "[Article][post]\n\n[post]: ../inline-svg/en.md#linked-heading",
        [{ kind: "article-link", text: "Article", suffix: "#linked-heading" }],
      );
      add(
        "article-link-html",
        '<a href="../inline-svg/en.md#linked-heading">Article</a>',
        [{ kind: "article-link", text: "Article", suffix: "#linked-heading" }],
      );
      add(
        "article-link-percent-encoded",
        "[Article](../inline%2Dsvg/en.md#linked-heading)",
        [{ kind: "article-link", text: "Article", suffix: "#linked-heading" }],
      );
      add(
        "article-link-encoded-extension",
        "[Article](../inline-svg/%65n.%6Dd#linked-heading)",
        [{ kind: "article-link", text: "Article", suffix: "#linked-heading" }],
      );
      for (const [label, url] of [
        ["percent", "100%25.svg"],
        ["hash", "a%23b.svg"],
        ["ampersand", "a%26b.svg"],
        ["apostrophe", "a%27b.svg"],
        ["double-encoded", "a%2526b.svg"],
        ...(process.platform === "win32" ? [] : [["question", "a%3Fb.svg"]]),
      ])
        add(
          "encoded-" + label,
          `![Encoded](./assets/${url})\n\n[Download](./assets/${url})\n\n<img src="./assets/${url}" alt="Raw">`,
          [image("Encoded"), link("Download"), raw("Raw")],
        );
      add(
        "literal-code",
        '```markdown\n![Example](./assets/missing.svg)\n[Download][pic]\n```\n\n`![Inline](./assets/missing.svg)`\n\n<!-- <img src="./assets/missing.svg"> -->',
        [],
      );
      add(
        "cached-cards",
        [
          "https://example.invalid/docs",
          "",
          "[https://example.invalid/docs][doc]",
          "",
          "> [!NOTE]",
          "> [https://example.invalid/docs][doc]",
          "",
          "[doc]: https://example.invalid/docs",
          "",
          "https://example.invalid/uncached",
          "",
          "[Meaningful label][doc]",
        ].join("\n"),
        [],
      );
      const proseCases = [
        "raw-density",
        "raw-width",
        "reference-image-and-link",
        "image-in-alert",
        "reference-space",
        "inline-space-angle",
        "reference-shortcut",
      ];
      let proseBody = proseCases
        .map((name) => {
          const c = cases.find((x) => x.name === name);
          return c.body.replaceAll("[pic]", "[" + name + "]");
        })
        .join("\n\n");
      proseBody +=
        '\n\n<label for="choice">Choice</label><input id="choice" aria-describedby="help"><p id="help">Help</p>\n\n<div id="details">Details</div><button aria-details="details">More</button>\n';
      const posts = cases.map((c) => {
        if (c.name === "inline-svg")
          c.body = "## Content\n\n## Linked heading\n\n" + c.body;
        const file = path.join(content, "posts/2026", c.name, "en.md");
        write(file, c.body);
        return {
          id: "posts/2026/" + c.name + "/en",
          file,
          body: c.body,
          group: "2026/" + c.name,
          year: "2026",
          articleKey: c.name,
          locale: "en",
          archive: { year: "2026", month: "09" },
          data: {
            title: c.name,
            slug: c.name,
            publishedAt: "2026-09-20T00:00:00Z",
            topics: ["notes"],
            summary: "Summary " + c.name,
            ...(c.name === "inline-svg"
              ? { aliases: ["/en/posts/old-inline-svg/"] }
              : {}),
          },
        };
      });
      const prose = ["home", "pages/about", "aside"].map((kind) => {
        const file = path.join(content, kind, "en.md");
        write(file, proseBody);
        return {
          id: kind + "/en",
          kind: kind === "pages/about" ? "about" : kind,
          locale: "en",
          file,
          body: proseBody,
        };
      });
      const assets = {
        "picture.svg": svg,
        "a b.svg": svg,
        "図.svg": svg,
        "nested/picture.svg": svg,
        "pixel.png": png,
        "100%.svg": svg,
        "a#b.svg": svg,
        "a&b.svg": svg,
        "a'b.svg": svg,
        "a%26b.svg": svg,
        ...(process.platform === "win32" ? {} : { "a?b.svg": svg }),
      };
      for (const post of posts)
        for (const [name, body] of Object.entries(assets)) {
          write(path.join(path.dirname(post.file), "assets", name), body);
          write(
            path.join(renderer, "public/media/2026", post.articleKey, name),
            body,
          );
        }
      for (const entry of prose)
        for (const [name, body] of Object.entries(assets)) {
          write(path.join(path.dirname(entry.file), "assets", name), body);
          write(path.join(renderer, "public/media", entry.kind, name), body);
        }
      write(
        path.join(work, "metadata.json"),
        JSON.stringify({ topics: { notes: { en: "Notes" } }, posts, prose }),
      );
      const site = {
        title: "Pattern review",
        baseUrl: "https://example.invalid",
        defaultLocale: "en",

        locales: { en: { label: "English" } },
        contentRoot: content,
        topicsFile: path.join(work, "topics.yaml"),
        linkcardsDir: path.join(work, "linkcards"),
        metadataFile: path.join(work, "metadata.json"),
        latestPosts: 5,
      };
      write(path.join(work, "site.json"), JSON.stringify(site));
      mkdirSync(site.linkcardsDir);
      write(
        path.join(site.linkcardsDir, "docs.json"),
        JSON.stringify({
          url: "https://example.invalid/docs",
          title: "Documentation",
          siteName: "Example",
          description: "Cached docs",
        }),
      );
      execFileSync(
        process.execPath,
        [path.join(root, "node_modules/astro/bin/astro.mjs"), "build"],
        {
          cwd: renderer,
          env: { ...process.env, CFGB_SITE_JSON: path.join(work, "site.json") },
          stdio: "pipe",
        },
      );
      function elements(html) {
        const out = [];
        const walk = (n) => {
          if (n.tagName) out.push(n);
          for (const c of n.childNodes ?? []) walk(c);
        };
        walk(parse(html));
        return out;
      }
      const attr = (node, name) =>
        node?.attrs?.find((a) => a.name === name)?.value;
      const textOf = (n) =>
        n.nodeName === "#text"
          ? n.value
          : (n.childNodes ?? []).map(textOf).join("");
      assert.ok(
        existsSync(path.join(work, "renderer", ".astro", "vite")),
        "Vite cache belongs to this workspace",
      );
      const dist = path.join(renderer, "dist");
      function checkPage(route, checks, scope = "content") {
        const html = readFileSync(path.join(dist, route, "index.html"), "utf8");
        const all = elements(html);
        const container =
          scope === "content"
            ? (all.find((n) => n.tagName === "article") ??
              all.find((n) => attr(n, "class") === "frame-main"))
            : all.find((n) => n.tagName === scope);
        const nodes = elementsFrom(container);
        const errors = [];
        const base = "https://example.invalid/" + route + "/";
        for (const check of checks) {
          if (check.kind === "srcset") {
            const element = nodes.find(
              (node) => node.tagName === check.tag && attr(node, "srcset"),
            );
            const prefix =
              scope === "content" && route.startsWith("en/posts/")
                ? `/media/2026/${route.split("/").at(-1)}/`
                : `/media/${scope === "content" ? (route === "en" ? "home" : route.split("/").at(-1)) : scope}/`;
            const expected = check.candidates
              .map(([url, descriptor]) => prefix + url + " " + descriptor)
              .join(", ");
            if (attr(element, "srcset") !== expected)
              errors.push("srcset mapping: " + attr(element, "srcset"));
            for (const [url] of check.candidates)
              validateUrl(prefix + url, "", "srcset", true);
          } else if (check.kind.includes("image")) {
            const img = nodes.find(
              (n) => n.tagName === "img" && attr(n, "alt") === check.alt,
            );
            if (check.kind === "image-link") {
              const anchor = nodes.find(
                (n) =>
                  n.tagName === "a" &&
                  (n.childNodes ?? []).some((c) => c.tagName === "img"),
              );
              const href = attr(anchor, "href");
              if (!href) errors.push("missing image link");
              else validateUrl(href, check.suffix, "image-link", true);
              continue;
            }
            if (!img) {
              errors.push("missing image " + check.alt);
              continue;
            }
            if (check.title && attr(img, "title") !== check.title)
              errors.push("image title " + check.alt);
            const src = attr(img, "src");
            if (!src) {
              errors.push("missing src " + check.alt);
              continue;
            }
            if (check.kind === "external-image") {
              if (src !== check.url) errors.push("external changed " + src);
              continue;
            }
            if (check.kind === "processed-image") {
              if (
                attr(img, "width") !== check.width ||
                attr(img, "height") !== check.height
              )
                errors.push("dimensions " + check.alt);
              if (src.startsWith("/media/") || src.startsWith("./assets/"))
                errors.push("pipeline bypass " + src);
            }
            validateUrl(
              src,
              check.suffix,
              check.alt,
              check.kind === "raw-image",
            );
          } else {
            const anchor = nodes.find(
              (n) => n.tagName === "a" && textOf(n) === check.text,
            );
            const href = attr(anchor, "href");
            if (check.title && attr(anchor, "title") !== check.title)
              errors.push("link title " + check.text);
            if (!href) {
              errors.push("missing link " + check.text);
              continue;
            }
            if (check.kind === "external-link") {
              if (href !== check.url)
                errors.push("external link changed " + href);
              continue;
            }
            validateUrl(
              href,
              check.suffix,
              check.text,
              check.kind !== "article-link",
            );
            if (
              check.kind === "article-link" &&
              !href.startsWith("/en/posts/inline-svg/")
            )
              errors.push("article route " + href);
          }
        }
        function validateUrl(value, suffix, label, media) {
          const url = new URL(value, base);
          let pathname;
          try {
            pathname = decodeURIComponent(url.pathname);
          } catch {
            errors.push("malformed escape " + value);
            return;
          }
          if (!existsSync(dist + pathname))
            errors.push("missing target " + label + ": " + url.pathname);
          if (
            url.hash &&
            (pathname.endsWith("/") || pathname.endsWith(".svg"))
          ) {
            const filename =
              dist + pathname + (pathname.endsWith("/") ? "index.html" : "");
            if (existsSync(filename)) {
              const id = decodeURIComponent(url.hash.slice(1));
              const targets = elements(readFileSync(filename, "utf8")).filter(
                (node) => attr(node, "id") === id,
              );
              if (targets.length !== 1)
                errors.push("fragment target " + label + ": " + value);
            }
          }
          if (suffix && url.search + url.hash !== suffix)
            errors.push("suffix lost " + label + ": " + value);
          if (media && !url.pathname.startsWith("/media/"))
            errors.push("media mapping " + label + ": " + value);
        }
        if (
          nodes.some((n) =>
            n.attrs?.some(
              (a) =>
                a.name === "__astro_image_" ||
                a.name === "data-cfgb-image-suffix",
            ),
          )
        )
          errors.push("unresolved image marker");
        return errors;
      }
      function elementsFrom(node) {
        const out = [];
        const visit = (n) => {
          if (!n) return;
          if (n.tagName) out.push(n);
          for (const c of n.childNodes ?? []) visit(c);
        };
        visit(node);
        return out;
      }
      const results = cases.map((c) => ({
        name: c.name,
        errors: checkPage("en/posts/" + c.name, c.checks),
      }));
      for (const kind of ["home", "about"])
        results.push({
          name: "prose-" + kind,
          errors: checkPage(
            kind === "home" ? "en" : "en/about",
            proseCases.flatMap(
              (name) => cases.find((x) => x.name === name).checks,
            ),
          ),
        });
      results.push({
        name: "prose-aside",
        errors: checkPage(
          "en/posts/inline-svg",
          proseCases.flatMap(
            (name) => cases.find((x) => x.name === name).checks,
          ),
          "aside",
        ),
      });
      for (const result of results)
        assert.deepEqual(
          result.errors,
          [],
          result.name + ": " + result.errors.join("; "),
        );
      const composed = elements(
        readFileSync(path.join(dist, "en/posts/inline-svg/index.html"), "utf8"),
      );
      const ids = composed.map((node) => attr(node, "id")).filter(Boolean);
      assert.equal(
        new Set(ids).size,
        ids.length,
        "layout, article, and aside IDs are distinct",
      );
      assert.ok(
        composed.some(
          (node) =>
            node.tagName === "a" &&
            attr(node, "class") === "skip" &&
            attr(node, "href") === "#cfgb-content",
        ),
      );
      const choice = composed.find((n) => attr(n, "id") === "aside-choice");
      assert.ok(choice);
      assert.ok(
        composed.some(
          (n) => n.tagName === "label" && attr(n, "for") === "aside-choice",
        ),
      );
      assert.ok(
        composed.some((n) => attr(n, "aria-details") === "aside-details"),
      );
      const cards = elements(
        readFileSync(
          path.join(dist, "en/posts/cached-cards/index.html"),
          "utf8",
        ),
      );
      const cached = cards.filter((node) => attr(node, "class") === "card");
      assert.equal(
        cached.length,
        3,
        "autolink, reference, and nested alert all become cards",
      );
      assert.ok(
        cached.every(
          (node) => attr(node, "href") === "https://example.invalid/docs",
        ),
      );
      assert.ok(
        cards.some(
          (node) => node.tagName === "a" && textOf(node) === "Meaningful label",
        ),
      );
      assert.ok(
        cards.some(
          (node) =>
            node.tagName === "a" &&
            attr(node, "href") === "https://example.invalid/uncached",
        ),
      );
      assert.equal(
        existsSync(path.join(dist, "en/posts/old-inline-svg/index.html")),
        false,
      );
      assert.ok(
        readFileSync(path.join(dist, "_redirects"), "utf8").includes(
          "/en/posts/old-inline-svg/ /en/posts/inline-svg/ 301",
        ),
      );
      const indexed = await createIndex();
      try {
        assert.deepEqual(indexed.errors, []);
        const scanned = await indexed.index.addDirectory({ path: dist });
        assert.deepEqual(scanned.errors, []);
        // addDirectory.page_count counts scanned HTML, including ignored pages.
        const generated = await indexed.index.getFiles();
        assert.deepEqual(generated.errors, []);
        const entry = generated.files.find(
          (file) => file.path === "pagefind-entry.json",
        );
        const search = JSON.parse(Buffer.from(entry.content).toString("utf8"));
        assert.equal(
          search.languages.en.page_count,
          posts.length,
          "only canonical articles enter the search index",
        );
        const fragments = generated.files
          .filter((file) => file.path.endsWith(".pf_fragment"))
          .map((file) => {
            const decoded = gunzipSync(file.content).toString("utf8");
            return JSON.parse(decoded.slice("pagefind_dcd".length));
          });
        assert.equal(fragments.length, posts.length);
        assert.ok(
          fragments.every((fragment) =>
            /^\/en\/posts\/[^/]+\/$/.test(fragment.url),
          ),
        );
        assert.equal(
          fragments.some((fragment) => fragment.url.includes("old-inline-svg")),
          false,
        );
        const imageOnly = fragments.find(
          (fragment) => fragment.url === "/en/posts/inline-raster/",
        );
        assert.ok(
          imageOnly.content.includes("inline-raster"),
          "image-only article title is indexed",
        );
        assert.equal(imageOnly.meta.summary, "Summary inline-raster");
        assert.ok(
          imageOnly.content.includes("Summary inline-raster"),
          "summary is indexed",
        );
        assert.ok(
          fragments.every((fragment) => !fragment.content.includes("Choice")),
          "aside text is excluded",
        );
        assert.equal(
          imageOnly.content.includes("Pattern review"),
          false,
          "layout brand is excluded",
        );
      } finally {
        await indexed.index.deleteIndex();
      }
      const literal = readFileSync(
        path.join(dist, "en/posts/literal-code/index.html"),
        "utf8",
      );
      assert.ok(literal.includes("./assets/missing.svg"));
      assert.ok(literal.includes('<!-- <img src="./assets/missing.svg"> -->'));
    } finally {
      rmSync(work, { recursive: true, force: true });
    }
  },
);
