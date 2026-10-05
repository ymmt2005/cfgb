import assert from "node:assert/strict";
import { after, before, describe, test } from "node:test";
import { startSite, withPage, largeDiagram } from "./fixture.mjs";

let site;
describe("Expanded Mermaid diagrams", { timeout: 180_000 }, () => {
  before(async () => {
    site = await startSite({ presentation: true });
  });
  after(async () => {
    await site?.close();
  });

  test("large diagrams expand, zoom, scroll and return keyboard focus", async () => {
    await withPage(
      site,
      { viewport: { width: 1280, height: 800 } },
      async (page) => {
        await page.goto(`${site.origin}/en/posts/large-diagram/`);
        await page.locator("pre.mermaid svg").waitFor();
        const expand = page.getByRole("button", {
          name: "Expand diagram",
          exact: true,
        });
        await expand.focus();
        await page.keyboard.press("Enter");
        const viewer = page.getByRole("dialog", {
          name: "Diagram",
          exact: true,
        });
        await viewer.waitFor();
        assert(await viewer.evaluate((el) => el.matches(":modal")));
        const bounds = await viewer.boundingBox();
        assert.equal(bounds.width, 1280);
        assert.equal(bounds.height, 800);
        assert.equal(await page.locator("pre.mermaid").count(), 1);
        assert.equal(
          await page.locator("pre.mermaid").getAttribute("data-source"),
          largeDiagram,
        );
        const svg = viewer.locator("svg");
        assert.equal(await viewer.locator("output").textContent(), "100%");
        await viewer
          .getByRole("button", { name: "Fit to window", exact: true })
          .click();
        const fit = (await svg.boundingBox()).width;
        await viewer
          .getByRole("button", { name: "Actual size", exact: true })
          .click();
        const actual = (await svg.boundingBox()).width;
        assert(actual > fit);
        const viewport = viewer.getByRole("region", {
          name: "Diagram",
          exact: true,
        });
        assert(
          await viewport.evaluate((el) => el.scrollWidth > el.clientWidth),
        );
        await viewport.focus();
        const initialScroll = await viewport.evaluate((el) => el.scrollLeft);
        await page.keyboard.press("ArrowRight");
        await page.waitForFunction(
          (previous) =>
            document.querySelector(".diagram-viewport").scrollLeft > previous,
          initialScroll,
        );
        await viewer
          .getByRole("button", { name: "Zoom in", exact: true })
          .click();
        assert((await svg.boundingBox()).width > actual);
        await viewer
          .getByRole("button", { name: "Zoom out", exact: true })
          .click();
        assert(Math.abs((await svg.boundingBox()).width - actual) < 1);
        await viewer
          .getByRole("button", { name: "Fit to window", exact: true })
          .click();
        assert(Math.abs((await svg.boundingBox()).width - fit) < 1);
        await page.keyboard.press("Escape");
        await viewer.waitFor({ state: "hidden" });
        assert(await expand.evaluate((el) => el === document.activeElement));
        assert.equal(
          await page.locator(".diagram-block pre.mermaid svg").count(),
          1,
        );
        await expand.click();
        await viewer
          .getByRole("button", { name: "Close diagram", exact: true })
          .click();
        await viewer.waitFor({ state: "hidden" });
        assert(await expand.evaluate((el) => el === document.activeElement));
      },
    );
  });

  test("native full screen can be entered, exited, and closed without leaving it active", async () => {
    await withPage(site, {}, async (page) => {
      await page.goto(`${site.origin}/en/posts/browser/`);
      await page
        .getByRole("button", { name: "Expand diagram", exact: true })
        .click();
      const viewer = page.getByRole("dialog", { name: "Diagram", exact: true });
      await viewer
        .getByRole("button", { name: "Full screen", exact: true })
        .click();
      await page.waitForFunction(() =>
        document.fullscreenElement?.classList.contains("diagram-viewer-shell"),
      );
      assert.equal(
        await viewer
          .getByRole("button", { name: "Exit full screen", exact: true })
          .getAttribute("aria-pressed"),
        "true",
      );
      await viewer
        .getByRole("button", { name: "Exit full screen", exact: true })
        .click();
      await page.waitForFunction(() => !document.fullscreenElement);
      assert(await viewer.isVisible());
      await viewer
        .getByRole("button", { name: "Full screen", exact: true })
        .click();
      await page.waitForFunction(() => !!document.fullscreenElement);
      await viewer
        .getByRole("button", { name: "Close diagram", exact: true })
        .click();
      await viewer.waitFor({ state: "hidden" });
      assert.equal(await page.evaluate(() => document.fullscreenElement), null);
      assert.equal(
        await page.locator(".diagram-block pre.mermaid svg").count(),
        1,
      );
      await page
        .getByRole("button", { name: "Expand diagram", exact: true })
        .click();
      await viewer
        .getByRole("button", { name: "Full screen", exact: true })
        .click();
      await page.waitForFunction(() => !!document.fullscreenElement);
      await page.keyboard.press("Escape");
      await viewer.waitFor({ state: "hidden" });
      assert.equal(await page.evaluate(() => document.fullscreenElement), null);
    });
  });

  test("expanded diagrams redraw with the system theme and retain manual zoom", async () => {
    await withPage(site, { colorScheme: "light" }, async (page) => {
      await page.goto(`${site.origin}/en/posts/browser/`);
      await page
        .getByRole("button", { name: "Expand diagram", exact: true })
        .click();
      const viewer = page.getByRole("dialog", { name: "Diagram", exact: true });
      await viewer
        .getByRole("button", { name: "Actual size", exact: true })
        .click();
      await viewer
        .getByRole("button", { name: "Zoom in", exact: true })
        .click();
      const label = await viewer.locator("output").textContent();
      const width = (await viewer.locator("svg").boundingBox()).width;
      const light = await viewer
        .locator("svg .node rect")
        .first()
        .evaluate((el) => getComputedStyle(el).fill);
      await page.emulateMedia({ colorScheme: "dark" });
      await page.waitForFunction((light) => {
        const node = document.querySelector(".diagram-viewer svg .node rect");
        return node && getComputedStyle(node).fill !== light;
      }, light);
      assert.equal(await viewer.locator("output").textContent(), label);
      assert(
        Math.abs((await viewer.locator("svg").boundingBox()).width - width) < 1,
      );
      await viewer
        .getByRole("button", { name: "Close diagram", exact: true })
        .click();
      await viewer.waitFor({ state: "hidden" });
      const dark = await page
        .locator(".diagram-block svg .node rect")
        .first()
        .evaluate((el) => getComputedStyle(el).fill);
      assert.notEqual(dark, light);
      assert.equal(await page.locator(".mermaid-error").count(), 0);
    });
  });

  test("mobile viewing is localized, fits on resize, and traps focus", async () => {
    for (const [locale, expand, title, close] of [
      ["en", "Expand diagram", "Diagram", "Close diagram"],
      ["ja", "図を拡大", "図", "図を閉じる"],
      ["zh-Hans", "展开图表", "图表", "关闭图表"],
      ["ko", "다이어그램 확대", "다이어그램", "다이어그램 닫기"],
    ]) {
      await withPage(
        site,
        {
          viewport: { width: 375, height: 812 },
          isMobile: true,
          hasTouch: true,
          colorScheme: "dark",
          reducedMotion: "reduce",
        },
        async (page) => {
          await page.goto(`${site.origin}/${locale}/posts/browser/`);
          const button = page.getByRole("button", {
            name: expand,
            exact: true,
          });
          await button.tap();
          const viewer = page.getByRole("dialog", { name: title, exact: true });
          await viewer.waitFor();
          const bounds = await viewer.boundingBox();
          assert.equal(bounds.width, 375);
          assert.equal(bounds.height, 812);
          const canvas = viewer.locator(".diagram-canvas");
          const viewport = viewer.locator(".diagram-viewport");
          assert(
            await viewport.evaluate(
              (el) => el.scrollWidth <= el.clientWidth + 1,
            ),
          );
          const before = (await canvas.boundingBox()).width;
          await page.setViewportSize({ width: 600, height: 375 });
          await page.waitForFunction(
            (before) =>
              document.querySelector(".diagram-canvas").getBoundingClientRect()
                .width !== before,
            before,
          );
          assert(
            await viewport.evaluate(
              (el) => el.scrollWidth <= el.clientWidth + 1,
            ),
          );
          await viewer
            .getByRole("button", { name: close, exact: true })
            .focus();
          await page.keyboard.press("Tab");
          assert(
            await viewer.evaluate((el) => el.contains(document.activeElement)),
          );
          await page.keyboard.press("Shift+Tab");
          assert(
            await viewer.evaluate((el) => el.contains(document.activeElement)),
          );
          await page.keyboard.press("Escape");
          await viewer.waitFor({ state: "hidden" });
          assert(await button.evaluate((el) => el === document.activeElement));
        },
      );
    }
  });

  test("multiple diagrams keep their SVG identities and can be opened independently", async () => {
    await withPage(site, {}, async (page) => {
      await page.goto(`${site.origin}/en/posts/presentation/`);
      await page.locator(".mermaid svg .actor").first().waitFor();
      const expand = page.getByRole("button", {
        name: "Expand diagram",
        exact: true,
      });
      assert.equal(await expand.count(), 2);
      for (let index = 0; index < 2; index++) {
        const id = await page
          .locator(".diagram-block pre.mermaid svg")
          .nth(index)
          .getAttribute("id");
        await expand.nth(index).click();
        const viewer = page.getByRole("dialog", {
          name: "Diagram",
          exact: true,
        });
        assert.equal(await viewer.locator("svg").getAttribute("id"), id);
        assert.equal(await page.locator("pre.mermaid svg").count(), 2);
        const duplicates = await page.evaluate(() => {
          const ids = [...document.querySelectorAll("[id]")].map((el) => el.id);
          return ids.filter((id, i) => ids.indexOf(id) !== i);
        });
        assert.deepEqual(duplicates, []);
        await viewer
          .getByRole("button", { name: "Close diagram", exact: true })
          .click();
        await viewer.waitFor({ state: "hidden" });
        assert.equal(
          await page
            .locator(".diagram-block pre.mermaid svg")
            .nth(index)
            .getAttribute("id"),
          id,
        );
      }
    });
  });

  test("a real fullscreen permissions policy retains the expanded fallback", async () => {
    const restricted = await startSite({
      enabledLanguages: ["en"],
      permissionsPolicy: "fullscreen=()",
    });
    try {
      await withPage(restricted, {}, async (page) => {
        await page.goto(`${restricted.origin}/en/posts/browser/`);
        assert.equal(
          await page.evaluate(() => document.fullscreenEnabled),
          false,
        );
        await page
          .getByRole("button", { name: "Expand diagram", exact: true })
          .click();
        const viewer = page.getByRole("dialog", {
          name: "Diagram",
          exact: true,
        });
        assert(await viewer.isVisible());
        assert.equal(
          await viewer.locator("[data-diagram-fullscreen]").isVisible(),
          false,
        );
        const before = (await viewer.locator("svg").boundingBox()).width;
        await viewer
          .getByRole("button", { name: "Zoom in", exact: true })
          .click();
        assert((await viewer.locator("svg").boundingBox()).width > before);
        await viewer
          .getByRole("button", { name: "Close diagram", exact: true })
          .click();
        await viewer.waitFor({ state: "hidden" });
      });
    } finally {
      await restricted.close();
    }
  });

  test("invalid diagrams and JavaScript-disabled source have no inactive controls", async () => {
    await withPage(site, {}, async (page) => {
      await page.goto(`${site.origin}/en/posts/diagram-error/`);
      await page.locator(".mermaid-error").waitFor();
      assert.equal(
        await page
          .getByRole("button", { name: "Expand diagram", exact: true })
          .count(),
        0,
      );
      assert.equal(await page.getByRole("dialog").count(), 0);
    });
    await withPage(site, { javaScriptEnabled: false }, async (page) => {
      await page.goto(`${site.origin}/en/posts/large-diagram/`);
      assert.equal(
        await page.locator("pre.mermaid").textContent(),
        largeDiagram,
      );
      assert.equal(
        await page
          .getByRole("button", { name: "Expand diagram", exact: true })
          .count(),
        0,
      );
      assert.equal(await page.getByRole("dialog").count(), 0);
    });
  });
});
