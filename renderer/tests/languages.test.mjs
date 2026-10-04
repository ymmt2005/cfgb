import assert from "node:assert/strict";
import test from "node:test";
import catalog from "../src/lib/locales.json" with { type: "json" };
import { formatDate, localeEntry } from "../src/lib/dates.mjs";

test("supported languages provide complete UI copy and localized UTC dates", () => {
  const keys = Object.keys(catalog.locales.en.ui).sort();
  for (const language of ["ja", "en", "zh-Hans", "ko"]) {
    const entry = localeEntry(language);
    assert.deepEqual(Object.keys(entry.ui).sort(), keys);
    for (const value of Object.values(entry.ui)) assert.ok(value.trim());
    assert.ok(formatDate("2026-09-30T16:30:00Z", language, "UTC"));
  }
  assert.equal(
    formatDate("2026-09-30T16:30:00Z", "zh-Hans", "UTC"),
    "2026年9月30日",
  );
  assert.equal(
    formatDate("2026-09-30T16:30:00Z", "ko", "UTC"),
    "2026년 9월 30일",
  );
  assert.throws(() => localeEntry("zh-hans"), /not supported/);
  assert.throws(() => localeEntry("ko-KR"), /not supported/);
});
