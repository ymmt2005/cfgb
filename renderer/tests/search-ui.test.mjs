import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { createContext, runInContext } from "node:vm";

test("search selects the document language as a locale filter", () => {
  const filters = [];
  const context = createContext({
    window: {
      PagefindUI: class {
        triggerFilters(value) {
          filters.push(value);
        }
      },
    },
    document: { documentElement: { lang: "en" } },
  });
  runInContext(readFileSync(new URL("../public/assets/search.js", import.meta.url), "utf8"), context);
  assert.equal(JSON.stringify(filters), JSON.stringify([{ locale: ["en"] }]));
});
