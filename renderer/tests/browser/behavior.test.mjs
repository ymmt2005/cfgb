import assert from "node:assert/strict";
import { after, before, describe, test } from "node:test";
import { code, diagram, startSite, withPage } from "./fixture.mjs";

let site;

describe(
  "browser APIs and progressive enhancement",
  { timeout: 180_000 },
  () => {
    before(async () => {
      site = await startSite();
    });
    after(async () => {
      await site?.close();
    });

    for (const deviceScaleFactor of [1, 2]) {
      test(`responsive images load native candidates at DPR ${deviceScaleFactor}`, async () => {
        await withPage(
          site,
          { deviceScaleFactor, viewport: { width: 1024, height: 768 } },
          async (page) => {
            await page.goto(`${site.origin}/en/posts/browser/`);
            for (const id of [
              "responsive-density",
              "responsive-width",
              "responsive-data",
            ]) {
              const image = page.locator(`#${id}`);
              await image.evaluate((el) => el.decode());
              const src = await image.evaluate((el) => el.currentSrc);
              if (id === "responsive-data" && deviceScaleFactor === 1)
                assert.ok(src.startsWith("data:image/svg+xml,"));
              else
                assert.equal(
                  src,
                  `${site.origin}/media/2026/browser/responsive-${deviceScaleFactor}.svg`,
                );
              assert.ok(await image.evaluate((el) => el.naturalWidth > 0));
            }
            const picture = page.locator("#responsive-picture");
            await picture.evaluate((el) => el.decode());
            assert.equal(
              await picture.evaluate((el) => el.currentSrc),
              `${site.origin}/media/2026/browser/responsive-1.svg`,
            );
            await page.setViewportSize({ width: 480, height: 768 });
            await page.waitForFunction(
              (expected) =>
                document.querySelector("#responsive-picture").currentSrc ===
                expected,
              `${site.origin}/media/2026/browser/responsive-2.svg`,
            );
            await picture.evaluate((el) => el.decode());
          },
        );
      });
    }

    test("saved theme paints before the interactive script loads", async () => {
      await withPage(site, { colorScheme: "light" }, async (page) => {
        await page.addInitScript(() =>
          localStorage.setItem("cfgb-theme", "dark"),
        );
        let release;
        const gate = new Promise((resolve) => {
          release = resolve;
        });
        await page.route("**/assets/site.js", async (route) => {
          await gate;
          await route.continue();
        });
        try {
          await page.goto(`${site.origin}/en/posts/browser/`, {
            waitUntil: "commit",
          });
          await page.locator(".article-title").waitFor();
          assert.equal(
            await page.locator("html").getAttribute("data-theme"),
            "dark",
          );
          assert.equal(
            await page
              .locator("html")
              .evaluate((el) => getComputedStyle(el).colorScheme),
            "dark",
          );
          assert.equal(
            await page
              .locator("html")
              .evaluate((el) => el.classList.contains("js")),
            false,
          );
        } finally {
          release();
        }
        await page.waitForFunction(() =>
          document.documentElement.classList.contains("js"),
        );
        assert.equal(
          await page.locator("html").getAttribute("data-theme"),
          "dark",
        );
      });
    });

    test("real Mermaid rendering follows system changes and explicit themes", async () => {
      await withPage(site, { colorScheme: "light" }, async (page) => {
        await page.goto(`${site.origin}/en/posts/browser/`);
        const node = page.locator("pre.mermaid svg .node rect").first();
        await node.waitFor();
        const light = await node.evaluate((el) => getComputedStyle(el).fill);
        assert.equal(
          await page.locator("pre.mermaid").getAttribute("data-source"),
          diagram,
        );
        await page.emulateMedia({ colorScheme: "dark" });
        await page.waitForFunction((light) => {
          const node = document.querySelector("pre.mermaid svg .node rect");
          return node && getComputedStyle(node).fill !== light;
        }, light);
        assert.equal(
          await page
            .locator("html")
            .evaluate((el) => getComputedStyle(el).colorScheme),
          "dark",
        );
        await chooseTheme(page, "light");
        await page.waitForFunction((light) => {
          const node = document.querySelector("pre.mermaid svg .node rect");
          return node && getComputedStyle(node).fill === light;
        }, light);
        const explicit = await page.locator("pre.mermaid").innerHTML();
        await page.emulateMedia({ colorScheme: "light" });
        await page.emulateMedia({ colorScheme: "dark" });
        await page.evaluate(
          () =>
            new Promise((resolve) =>
              requestAnimationFrame(() => requestAnimationFrame(resolve)),
            ),
        );
        assert.equal(await page.locator("pre.mermaid").innerHTML(), explicit);
        assert.equal(
          await page
            .locator("html")
            .evaluate((el) => getComputedStyle(el).colorScheme),
          "light",
        );
        // Rapid real selections exercise the queue without replacing Mermaid.
        const previousId = await page
          .locator("pre.mermaid svg")
          .getAttribute("id");
        await chooseTheme(page, "dark");
        await chooseTheme(page, "light");
        await page.waitForFunction(
          ({ light, previousId }) => {
            const node = document.querySelector("pre.mermaid svg .node rect");
            return (
              node &&
              node.closest("svg").id !== previousId &&
              getComputedStyle(node).fill === light
            );
          },
          { light, previousId },
        );
        assert.equal(
          await page.locator("pre.mermaid").getAttribute("data-source"),
          diagram,
        );
        assert.equal(await page.locator(".mermaid-error").count(), 0);
      });
    });

    test("invalid Mermaid keeps its source and a single error after redraw", async () => {
      await withPage(site, {}, async (page) => {
        await page.goto(`${site.origin}/en/posts/diagram-error/`);
        const note = page.locator("pre.mermaid + .mermaid-error");
        await note.waitFor();
        assert.equal(
          await page.locator("pre.mermaid").textContent(),
          "this is not a diagram",
        );
        for (const theme of ["dark", "light"]) {
          const previous = await note.elementHandle();
          await chooseTheme(page, theme);
          await previous.waitForElementState("hidden");
          await previous.dispose();
          await note.waitFor();
          assert.equal(await page.locator(".mermaid-error").count(), 1);
          assert.equal(
            await page.locator("pre.mermaid").textContent(),
            "this is not a diagram",
          );
          assert.ok((await note.textContent()).length > 0);
        }
      });
    });

    test("code copy writes exact multiline Unicode text to the real clipboard", async () => {
      await withPage(site, {}, async (page, context) => {
        await context.grantPermissions(["clipboard-read", "clipboard-write"], {
          origin: site.origin,
        });
        await page.goto(`${site.origin}/en/posts/browser/`);
        const copy = page.getByRole("button", { name: "Copy to clipboard" });
        await copy.click();
        await page.waitForFunction(
          (code) =>
            navigator.clipboard.readText().then((text) => text === code),
          code,
        );
        assert.equal(
          await page.evaluate(() => navigator.clipboard.readText()),
          code,
        );
        await page.locator(".expressive-code .feedback").waitFor();
        assert.ok(
          (await page.locator(".expressive-code .feedback").textContent())
            .length > 0,
        );
      });
    });

    for (const locale of ["ja", "en"]) {
      test(`Pagefind ${locale} search runs WASM, restricts locale and navigates results`, async () => {
        await withPage(
          site,
          {
            locale: locale === "ja" ? "en-US" : "ja-JP",
            viewport: { width: 1440, height: 900 },
            reducedMotion: "reduce",
          },
          async (page) => {
            const wasm = [];
            page.on("response", (response) => {
              if (
                new URL(response.url()).pathname.startsWith("/pagefind/wasm.")
              )
                wasm.push(response.status());
            });
            await page.goto(`${site.origin}/${locale}/search/`);
            const input = page.locator(".pagefind-ui__search-input");
            await input.fill("browserneedle");
            await page.waitForFunction((locale) => {
              const links = [
                ...document.querySelectorAll(
                  ".pagefind-ui__result-inner > .pagefind-ui__result-title > a",
                ),
              ];
              return (
                links.length === 1 &&
                new URL(links[0].href).pathname === `/${locale}/posts/browser/`
              );
            }, locale);
            const result = page.locator(
              ".pagefind-ui__result-inner > .pagefind-ui__result-title > a",
            );
            assert.equal(await result.count(), 1);
            assert.deepEqual(
              wasm,
              [200],
              "the actual locale WASM bundle loaded",
            );
            const heading = page.locator(".pagefind-ui__result-nested a");
            await heading.click();
            await page.waitForURL(
              `${site.origin}/${locale}/posts/browser/#normal-heading`,
            );
            const target = await page.locator("#normal-heading").boundingBox();
            const header = await page.locator(".mast").boundingBox();
            assert.ok(
              target.y >= header.y + header.height && target.y < 900,
              "search fragment is readable below the sticky masthead",
            );
            await page.goBack();
            // Browser history returns to the search page; a new query is a
            // separate interaction, not an assumption about cached UI state.
            await input.fill("browserneedle");
            await result.click();
            await page.waitForURL(`${site.origin}/${locale}/posts/browser/`);
            assert.equal(
              await page.locator("html").getAttribute("lang"),
              locale,
            );
            await page.goBack();
            await input.fill("zzzznothingmatches");
            await page.waitForFunction(
              () =>
                document
                  .querySelector(".pagefind-ui__message")
                  ?.textContent.includes("zzzznothingmatches") &&
                document.querySelectorAll(
                  ".pagefind-ui__result-inner > .pagefind-ui__result-title > a",
                ).length === 0,
            );
            await input.fill("browserneedle");
            await result.waitFor();
            assert.equal(
              new URL(await result.getAttribute("href"), site.origin).pathname,
              `/${locale}/posts/browser/`,
            );
          },
        );
      });
    }

    for (const [name, width, js] of [
      ["desktop", 1440, true],
      ["mobile without JavaScript", 375, false],
    ]) {
      test(`native keyboard, TOC and footnote navigation on ${name}`, async () => {
        await withPage(
          site,
          {
            viewport: { width, height: 900 },
            javaScriptEnabled: js,
            reducedMotion: "reduce",
          },
          async (page) => {
            await page.goto(`${site.origin}/en/posts/browser/`);
            await page.keyboard.press("Tab");
            const skip = page.getByRole("link", { name: "Skip to content" });
            assert.equal(
              await skip.evaluate((el) => el === document.activeElement),
              true,
            );
            assert.ok(
              (await skip.boundingBox()).y >= 0,
              "skip link is painted on focus",
            );
            await page.keyboard.press("Enter");
            await page.waitForURL(/#cfgb-content$/);
            await page.keyboard.press("Tab");
            assert.equal(
              await page
                .locator("main")
                .evaluate((el) => el.contains(document.activeElement)),
              true,
              "skip moves the keyboard starting point past the header",
            );
            const toc = page.locator(
              width === 1440 ? ".toc-desktop" : ".toc-mobile",
            );
            const other = page.locator(
              width === 1440 ? ".toc-mobile" : ".toc-desktop",
            );
            assert.equal(await toc.isVisible(), true);
            assert.equal(await other.isVisible(), false);
            const mainWidth = (await page.locator(".frame-main").boundingBox())
              .width;
            await toc.locator("summary").focus();
            await page.keyboard.press("Enter");
            assert.equal(await toc.getAttribute("open"), null);
            assert.equal(await toc.locator("ol").isVisible(), false);
            if (width === 1440)
              assert.ok(
                (await page.locator(".frame-main").boundingBox()).width >
                  mainWidth,
                "collapsed TOC releases its column",
              );
            await page.keyboard.press("Enter");
            await toc.getByRole("link", { name: "Normal heading" }).click();
            await page.waitForURL(/#normal-heading$/);
            const target = await page.locator("#normal-heading").boundingBox();
            assert.ok(
              target.y >= 0 && target.y < 900,
              "native anchor is in the viewport",
            );
            if (width === 1440) {
              const header = await page.locator(".mast").boundingBox();
              assert.ok(
                target.y >= header.y + header.height,
                "TOC target is not covered by the sticky masthead",
              );
            }
            const ref = page.locator("[data-footnote-ref]");
            const reference = await ref.getAttribute("id");
            const footnote = await ref.getAttribute("href");
            await ref.click();
            await page.waitForURL(
              (url) =>
                decodeURIComponent(url.hash) === decodeURIComponent(footnote),
            );
            await page.locator("[data-footnote-backref]").click();
            await page.waitForURL(
              (url) => decodeURIComponent(url.hash.slice(1)) === reference,
            );
            assert.ok(
              await page.evaluate(
                () => document.documentElement.scrollWidth <= window.innerWidth,
              ),
              "no page-wide horizontal overflow",
            );
            assert.equal(
              await page
                .locator("html")
                .evaluate((el) => getComputedStyle(el).scrollBehavior),
              "auto",
            );
            if (!js) {
              assert.equal(await page.locator("pre.mermaid svg").count(), 0);
              assert.equal(
                await page.locator("pre.mermaid").textContent(),
                diagram,
              );
              assert.equal(
                await page
                  .locator("html")
                  .evaluate((el) => el.classList.contains("js")),
                false,
              );
              await page
                .getByRole("navigation", { name: "Site" })
                .getByRole("link", { name: "Posts", exact: true })
                .click();
              await page.waitForURL(`${site.origin}/en/posts/`);
            }
          },
        );
      });
    }

    test("theme menu dismisses on outside pointer and keyboard focus departure", async () => {
      await withPage(site, {}, async (page) => {
        await page.goto(`${site.origin}/en/posts/browser/`);
        const button = page.getByRole("button", { name: "Theme", exact: true });
        const list = page.getByRole("listbox", { name: "Theme", exact: true });
        await button.click();
        await page.locator(".article-title").click();
        assert.equal(await list.isVisible(), false);
        assert.equal(await button.getAttribute("aria-expanded"), "false");
        await button.click();
        await page.keyboard.press("Tab");
        assert.equal(await list.isVisible(), false);
        assert.equal(await button.getAttribute("aria-expanded"), "false");
        assert.equal(
          await page
            .locator(".menu")
            .evaluate((el) => el.contains(document.activeElement)),
          false,
          "Tab proceeds to the next control outside the menu",
        );
      });
    });
  },
);

async function chooseTheme(page, value) {
  await page.getByRole("button", { name: "Theme", exact: true }).click();
  await page.locator(`[role="option"][data-value="${value}"]`).click();
}
