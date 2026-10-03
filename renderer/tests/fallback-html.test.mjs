import assert from "node:assert/strict";
import test from "node:test";
import { fallbackHtml } from "../src/lib/fallback-html.mjs";

test("root 404 lists only configured locales and escapes labels", () => {
  const html = fallbackHtml(
    {
      site: {
        title: "Example & Co",
        defaultLocale: "ja",
        locales: { ja: { label: '日本語 <b>' } },
      },
    },
    "both",
  );
  assert.match(html, /href="\/ja\/"/);
  assert.match(html, /href="\/ja\/search\/"/);
  assert.doesNotMatch(html, /href="\/en\//);
  assert.match(html, /日本語 &lt;b&gt;/);
  assert.match(html, /Example &amp; Co/);
  assert.doesNotMatch(html, /日本語 <b>/);
  assert.match(html, /<body data-pagefind-ignore="all">/);
});
