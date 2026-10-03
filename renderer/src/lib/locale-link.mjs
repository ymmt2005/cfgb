// Translation helpers take collections. The shipped header still shows one
// other language; a selector for more languages is separate work.

// groupCounterparts maps each locale in an article group to its own URL.
// Another group that uses the same slug is not a member.
export function groupCounterparts(posts, group) {
  const counterparts = {};
  for (const post of posts) {
    if (post.group === group) counterparts[post.locale] = post.url;
  }
  return counterparts;
}

export function pageCounterparts(locales, pathFor) {
  return Object.fromEntries(locales.map((locale) => [locale, pathFor(locale)]));
}

// languageDestination resolves one requested locale.
// A counterpart map wins. A missing article translation is that locale's home.
// Pages without a map follow a route that exists, then the locale home.
export function languageDestination({ path, locale, destination, counterparts, routes }) {
  if (!destination) return "";
  if (counterparts) {
    if (Object.hasOwn(counterparts, destination)) return counterparts[destination];
    return `/${destination}/`;
  }
  const swapped = swapLocale(path, locale, destination);
  if (routes?.has(swapped)) return swapped;
  return `/${destination}/`;
}

// alternateMembers lists a real translation group. A single member has no alternates.
export function alternateMembers(counterparts) {
  const members = Object.entries(counterparts || {}).map(([locale, href]) => ({ locale, href }));
  if (members.length < 2) return [];
  members.sort((left, right) => (left.locale < right.locale ? -1 : left.locale > right.locale ? 1 : 0));
  return members;
}

export function swapLocale(route, from, to) {
  if (route === `/${from}/` || route.startsWith(`/${from}/`)) return `/${to}/${route.slice(from.length + 2)}`;
  return `/${to}/`;
}
