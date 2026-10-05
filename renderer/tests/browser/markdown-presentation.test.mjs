import assert from "node:assert/strict";
import { after, before, describe, test } from "node:test";
import { startSite, withPage } from "./fixture.mjs";

let site;
describe("Markdown presentation from the mockup", { timeout: 180_000 }, () => {
  before(async () => {
    site = await startSite({ enabledLanguages: ["en"], presentation: true });
  });
  after(async () => {
    await site?.close();
  });

  test("wide GFM tables scroll inside the reading column on mobile", async () => {
    await withPage(
      site,
      { viewport: { width: 375, height: 812 } },
      async (page) => {
        await page.goto(`${site.origin}/en/posts/presentation/`);
        const table = page.locator(".table-wrap");
        assert.equal(await table.count(), 1);
        assert.ok(
          await table.evaluate((el) => el.scrollWidth > el.clientWidth),
        );
        await table.evaluate((el) => (el.scrollLeft = el.scrollWidth));
        assert.ok(await table.evaluate((el) => el.scrollLeft > 0));
        assert.ok(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth + 1,
          ),
        );
      },
    );
  });

  test("footnotes keep compact superscripts, accessible IDs and native return navigation", async () => {
    await withPage(site, { javaScriptEnabled: false }, async (page) => {
      await page.goto(`${site.origin}/en/posts/presentation/`);
      const reference = page.locator(".prose sup.footnote-ref");
      assert.equal(await reference.count(), 1);
      const style = await reference.evaluate((el) => ({
        height: getComputedStyle(el).lineHeight,
        size: parseFloat(getComputedStyle(el).fontSize),
        parentSize: parseFloat(getComputedStyle(el.parentElement).fontSize),
      }));
      assert.equal(style.height, "0px");
      assert.ok(Math.abs(style.size / style.parentSize - 0.72) < 0.01);
      const link = reference.locator("a[data-footnote-ref]");
      const target = await link.getAttribute("href");
      await link.click();
      await page.waitForURL((url) => url.hash === target);
      const back = page.locator(
        ".footnotes a.footnote-back[data-footnote-backref]",
      );
      assert.equal(await back.count(), 1);
      assert.ok(await back.getAttribute("aria-label"));
      const returnTo = await back.getAttribute("href");
      await back.click();
      await page.waitForURL((url) => url.hash === returnTo);
    });
  });

  test("Mermaid frames, fonts and labels follow the page palette in light and dark", async () => {
    for (const colorScheme of ["light", "dark"]) {
      await withPage(
        site,
        { colorScheme, viewport: { width: 375, height: 812 } },
        async (page) => {
          await page.goto(`${site.origin}/en/posts/presentation/`);
          await page.locator(".mermaid svg .nodeLabel").first().waitFor();
          await page.locator(".mermaid svg .actor").first().waitFor();
          const colors = await page.evaluate(() => {
            const probe = document.createElement("span");
            document.body.append(probe);
            const token = (name) => {
              probe.style.color = `var(--${name})`;
              return getComputedStyle(probe).color;
            };
            const expected = {
              ink: token("ink"),
              soft: token("soft"),
              raised: token("bg-raised"),
              line: token("line"),
            };
            probe.remove();
            return {
              expected,
              font: getComputedStyle(document.querySelector(".prose"))
                .fontFamily,
              frames: [...document.querySelectorAll(".diagram-block")].map(
                (el) => {
                  const c = getComputedStyle(el);
                  return {
                    background: c.backgroundColor,
                    border: c.borderTopColor,
                    radius: c.borderRadius,
                  };
                },
              ),
              label: [...document.querySelectorAll(".mermaid .nodeLabel")].map(
                (el) => ({
                  color: getComputedStyle(el).color,
                  font: getComputedStyle(el).fontFamily,
                }),
              ),
              shadows: [
                ...document.querySelectorAll(
                  ".mermaid .node rect, .mermaid rect.actor",
                ),
              ].map((el) => getComputedStyle(el).filter),
              nodes: [...document.querySelectorAll(".mermaid .node rect")].map(
                (el) => getComputedStyle(el).fill,
              ),
              sequenceFonts: [
                ...document.querySelectorAll(
                  ".mermaid text.actor > tspan, .mermaid .messageText, .mermaid .noteText",
                ),
              ].map((el) => getComputedStyle(el).fontFamily),
              actors: [
                ...document.querySelectorAll(".mermaid text.actor > tspan"),
              ].map((el) => getComputedStyle(el).fill),
              signals: [
                ...document.querySelectorAll(".mermaid .messageText"),
              ].map((el) => getComputedStyle(el).fill),
            };
          });
          for (const frame of colors.frames) {
            assert.equal(frame.background, colors.expected.raised);
            assert.equal(frame.border, colors.expected.line);
            assert.equal(frame.radius, "6.4px");
          }
          for (const label of colors.label) {
            assert.equal(label.color, colors.expected.ink);
            assert.equal(label.font, colors.font);
          }
          for (const font of colors.sequenceFonts)
            assert.equal(font, colors.font);
          assert.ok(
            colors.shadows.some((shadow) => shadow !== "none"),
            "retain Mermaid's default drop shadows",
          );
          assert.ok(colors.nodes.length > 0);
          for (const fill of colors.nodes)
            assert.equal(fill, colors.expected.soft);
          assert.ok(colors.actors.length > 0);
          assert.ok(colors.signals.length > 0);
          for (const fill of [...colors.actors, ...colors.signals])
            assert.equal(fill, colors.expected.ink);
          const frame = page.locator(".diagram-block").first();
          assert.ok(
            await frame.evaluate((el) => el.scrollWidth > el.clientWidth),
            "wide diagram keeps readable natural dimensions",
          );
          await frame.evaluate((el) => (el.scrollLeft = el.scrollWidth));
          assert.ok(await frame.evaluate((el) => el.scrollLeft > 0));
          assert.ok(
            await page.evaluate(
              () => document.documentElement.scrollWidth <= innerWidth + 1,
            ),
          );
        },
      );
    }
  });

  test("prose, alerts, lists, images, cards and disclosures retain the mockup styling", async () => {
    for (const colorScheme of ["light", "dark"])
      await withPage(site, { colorScheme }, async (page) => {
        await page.goto(`${site.origin}/en/posts/presentation/`);
        const heading = page.locator(".prose h2").first();
        assert.equal(
          await heading.evaluate((el) => getComputedStyle(el).fontSize),
          "20px",
        );
        for (const kind of ["note", "tip", "important", "warning", "caution"]) {
          const alert = page.locator(`.alert-${kind}`);
          assert.equal(await alert.count(), 1);
          const styles = await alert.evaluate((el) => {
            const c = getComputedStyle(el),
              label = getComputedStyle(el.querySelector(".alert-label"));
            return {
              border: c.borderLeftWidth,
              color: c.borderLeftColor,
              label: label.color,
              padding: c.padding,
              bold: el.querySelector("strong") !== null,
            };
          });
          assert.equal(styles.border, "3px");
          assert.equal(styles.color, styles.label);
          assert.equal(styles.padding, "11.2px 14.4px");
          assert.equal(styles.bold, true);
        }
        assert.equal(
          await page.locator(".prose blockquote:not(.alert)").count(),
          1,
        );
        assert.equal(await page.locator(".prose em").count(), 1);
        assert.ok((await page.locator(".prose code").count()) > 0);
        assert.equal(
          await page.locator('.prose input[type="checkbox"][disabled]').count(),
          2,
        );
        const image = page.getByAltText("Markdown presentation image");
        await image.evaluate((el) => el.decode());
        assert.equal(
          await image.evaluate((el) => getComputedStyle(el).borderRadius),
          "6.4px",
        );
        const card = page.locator(".card");
        assert.equal(
          await card.locator(".card-title").textContent(),
          "Cached link card",
        );
        assert.equal(
          await card.evaluate((el) => getComputedStyle(el).borderRadius),
          "7.2px",
        );
        const disclosure = page.locator(".prose details");
        await disclosure.locator("summary").click();
        assert.equal(await disclosure.getAttribute("open"), "");
      });
  });

  test("without JavaScript diagrams retain their original readable source", async () => {
    await withPage(
      site,
      { javaScriptEnabled: false, viewport: { width: 375, height: 812 } },
      async (page) => {
        await page.goto(`${site.origin}/en/posts/presentation/`);
        assert.equal(await page.locator(".mermaid svg").count(), 0);
        assert.equal(await page.locator("pre.mermaid").count(), 2);
        assert.match(
          await page.locator("pre.mermaid").first().textContent(),
          /flowchart LR/,
        );
        assert.equal(
          await page
            .locator("pre.mermaid")
            .first()
            .evaluate((el) => getComputedStyle(el).fontSize),
          "13.6px",
        );
        assert.ok(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth + 1,
          ),
        );
      },
    );
  });
});
