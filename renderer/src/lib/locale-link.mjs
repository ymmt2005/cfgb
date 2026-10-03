// languageDestination chooses the other-locale page for the language switch.
// A real translation uses its group counterpart. An untranslated article goes
// to the other locale home, even when a different group publishes the same slug.
// Other pages follow a route that actually exists.
export function languageDestination({ path, locale, other, alternatePath = "", untranslated = false, routes }) {
  if (!other) return "";
  if (alternatePath) return alternatePath;
  if (untranslated) return `/${other}/`;
  const swapped = swapLocale(path, locale, other);
  if (routes?.has(swapped)) return swapped;
  return `/${other}/`;
}

export function swapLocale(route, from, to) {
  if (route === `/${from}/` || route.startsWith(`/${from}/`)) return `/${to}/${route.slice(from.length + 2)}`;
  return `/${to}/`;
}
