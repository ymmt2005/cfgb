import assert from "node:assert/strict";
import { after, before, describe, test } from "node:test";
import { startSite } from "./fixture.mjs";

let site, browser, origin;

describe("generated site interactions", { timeout: 180_000 }, () => {
  before(async () => {
    site = await startSite();
    ({ browser, origin } = site);
  });
  after(async () => {
    await site?.close();
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
