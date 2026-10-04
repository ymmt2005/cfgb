import assert from "node:assert/strict";
import test from "node:test";
import { alternateMembers, groupCounterparts, languageDestination, pageCounterparts } from "../src/lib/locale-link.mjs";

const routes = new Set([
  "/ja/",
  "/en/",
  "/pt-BR/",
  "/ja/posts/",
  "/en/posts/",
  "/ja/search/",
  "/en/search/",
  "/ja/posts/shared/",
  "/en/posts/shared/",
  "/pt-BR/posts/shared/",
  "/ja/about/",
  "/en/about/",
]);

const posts = [
  { group: "2026/alpha", locale: "ja", url: "/ja/posts/shared/" },
  { group: "2026/alpha", locale: "en", url: "/en/posts/reading/" },
  { group: "2026/beta", locale: "pt-BR", url: "/pt-BR/posts/shared/" },
];

test("an untranslated article ignores another group's same slug", () => {
  const counterparts = groupCounterparts(posts, "2026/beta");
  assert.deepEqual(counterparts, { "pt-BR": "/pt-BR/posts/shared/" });
  assert.deepEqual(alternateMembers(counterparts), []);
  const destination = languageDestination({
    path: "/pt-BR/posts/shared/",
    locale: "pt-BR",
    destination: "en",
    counterparts,
    routes,
  });
  assert.equal(destination, "/en/");
});

test("a translated article uses its group counterpart", () => {
  const counterparts = groupCounterparts(posts, "2026/alpha");
  assert.deepEqual(counterparts, {
    ja: "/ja/posts/shared/",
    en: "/en/posts/reading/",
  });
  assert.deepEqual(
    alternateMembers(counterparts).map((item) => item.locale),
    ["en", "ja"],
  );
  const destination = languageDestination({
    path: "/ja/posts/shared/",
    locale: "ja",
    destination: "en",
    counterparts,
    routes,
  });
  assert.equal(destination, "/en/posts/reading/");
});

test("a missing translation does not select another group's same slug", () => {
  const counterparts = groupCounterparts(posts, "2026/alpha");
  const destination = languageDestination({
    path: "/ja/posts/shared/",
    locale: "ja",
    destination: "pt-BR",
    counterparts,
    routes,
  });
  assert.equal(destination, "/pt-BR/");
  assert.equal(counterparts["pt-BR"], undefined);
});

test("a non-article page keeps a route that exists in the other locale", () => {
  const destination = languageDestination({
    path: "/ja/search/",
    locale: "ja",
    destination: "en",
    routes,
  });
  assert.equal(destination, "/en/search/");
});

test("a missing counterpart route falls back to the other locale home", () => {
  const destination = languageDestination({
    path: "/ja/topics/only-ja/",
    locale: "ja",
    destination: "en",
    routes,
  });
  assert.equal(destination, "/en/");
});

test("page counterparts cover every configured locale", () => {
  const counterparts = pageCounterparts(["ja", "en", "pt-BR"], (locale) => `/${locale}/about/`);
  assert.deepEqual(counterparts, {
    ja: "/ja/about/",
    en: "/en/about/",
    "pt-BR": "/pt-BR/about/",
  });
  assert.equal(
    languageDestination({
      path: "/ja/about/",
      locale: "ja",
      destination: "pt-BR",
      counterparts,
      routes,
    }),
    "/pt-BR/about/",
  );
});
