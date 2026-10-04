// Domain routes stay relative to the blog root. Only emitted URLs carry the
// hosting prefix, so translation/routing logic is independent of the host.
export function basePath(site) {
  return site.baseUrl ? new URL(site.baseUrl).pathname.replace(/\/$/, "") : "";
}

export function sitePath(site, route) {
  if (!route.startsWith("/") || route.startsWith("//")) return route;
  return basePath(site) + route;
}

export function routePath(site, pathname) {
  const base = basePath(site);
  return base && pathname.startsWith(`${base}/`)
    ? pathname.slice(base.length)
    : pathname;
}

export function absolute(site, route) {
  return new URL(sitePath(site, route), new URL(site.baseUrl).origin).href;
}
