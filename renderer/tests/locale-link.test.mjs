import assert from "node:assert/strict";
import test from "node:test";
import { languageDestination } from "../src/lib/locale-link.mjs";

const routes = new Set([
  "/ja/",
  "/en/",
  "/ja/posts/",
  "/en/posts/",
  "/ja/search/",
  "/en/search/",
  "/ja/posts/shared/",
  "/en/posts/shared/",
  "/ja/about/",
  "/en/about/",
]);

test("an untranslated article ignores another group's same slug", () => {
  const destination = languageDestination({
    path: "/ja/posts/shared/",
    locale: "ja",
    other: "en",
    alternatePath: "",
    untranslated: true,
    routes,
  });
  assert.equal(destination, "/en/");
});

test("a translated article uses its group counterpart", () => {
  const destination = languageDestination({
    path: "/ja/posts/shared/",
    locale: "ja",
    other: "en",
    alternatePath: "/en/posts/reading-protobuf-schemas/",
    untranslated: false,
    routes,
  });
  assert.equal(destination, "/en/posts/reading-protobuf-schemas/");
});

test("a non-article page keeps a route that exists in the other locale", () => {
  const destination = languageDestination({
    path: "/ja/search/",
    locale: "ja",
    other: "en",
    routes,
  });
  assert.equal(destination, "/en/search/");
});

test("a missing counterpart route falls back to the other locale home", () => {
  const destination = languageDestination({
    path: "/ja/topics/only-ja/",
    locale: "ja",
    other: "en",
    routes,
  });
  assert.equal(destination, "/en/");
});
