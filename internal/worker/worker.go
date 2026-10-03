// Package worker renders the locale-negotiation Worker shipped in a build artifact.
package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Options are baked into the generated Worker.
type Options struct {
	DefaultLocale string
	Locales       []string
	Routes        []string
}

// Source returns a single ES module.
func Source(opts Options) ([]byte, error) {
	if opts.DefaultLocale == "" || len(opts.Locales) == 0 {
		return nil, fmt.Errorf("worker requires a default locale")
	}
	payload, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("const site = ")
	buf.Write(payload)
	buf.WriteString(";\n")
	buf.WriteString(runtime)
	return buf.Bytes(), nil
}

const runtime = `
const routes = new Set(site.Routes);

function localeFromCookie(header) {
  if (!header) return "";
  const parts = header.split(";");
  for (const part of parts) {
    const [name, value] = part.trim().split("=");
    if (name === "cfgb_locale") return value || "";
  }
  return "";
}

function negotiatedLocale(header) {
  const ranked = [];
  if (header) {
    header.split(",").forEach((piece, index) => {
      const [tagPart, ...params] = piece.trim().split(";");
      const tag = tagPart.trim().toLowerCase();
      if (!tag) return;
      let quality = 1;
      for (const param of params) {
        const [key, raw] = param.trim().split("=");
        if (key === "q") quality = Number(raw);
      }
      if (!Number.isFinite(quality) || quality <= 0) return;
      ranked.push({ tag, quality, index });
    });
  }
  ranked.sort((a, b) => b.quality - a.quality || a.index - b.index);
  for (const item of ranked) {
    for (const locale of site.Locales) {
      if (item.tag === locale || item.tag.startsWith(locale + "-")) return locale;
    }
  }
  return site.DefaultLocale;
}

function safeNext(raw, locale) {
  const home = "/" + locale + "/";
  if (raw == null || raw === "") return home;
  if (raw.includes("\\") || /[\u0000-\u001f\u007f]/.test(raw)) return home;
  if (!raw.startsWith("/") || raw.startsWith("//") || raw.includes("://")) return home;
  let decoded;
  try { decoded = decodeURIComponent(raw); } catch (error) { return home; }
  if (decoded.includes("\\") || /[\u0000-\u001f\u007f]/.test(decoded)) return home;
  if (!decoded.startsWith("/") || decoded.startsWith("//") || decoded.includes("://")) return home;
  if (decoded.split("/").some((part) => part === "." || part === "..")) return home;
  if (!decoded.startsWith(home) || !routes.has(decoded)) return home;
  return decoded;
}

function localeResponse(request) {
  const url = new URL(request.url);
  if (request.method !== "GET") {
    return new Response("method not allowed\n", { status: 405, headers: { "cache-control": "no-store", allow: "GET" } });
  }
  const lang = url.searchParams.get("lang") || "";
  if (!site.Locales.includes(lang)) {
    return new Response("unsupported locale\n", { status: 400, headers: { "cache-control": "no-store" } });
  }
  const next = safeNext(url.searchParams.get("next"), lang);
  const headers = new Headers({
    location: next,
    "cache-control": "no-store",
    "set-cookie": "cfgb_locale=" + lang + "; Secure; HttpOnly; SameSite=Lax; Path=/; Max-Age=31536000",
  });
  return new Response(null, { status: 303, headers });
}

function rootResponse(request) {
  if (request.method !== "GET" && request.method !== "HEAD") {
    return new Response("method not allowed\n", { status: 405, headers: { "cache-control": "private, no-store", allow: "GET, HEAD" } });
  }
  const cookie = localeFromCookie(request.headers.get("cookie"));
  const locale = site.Locales.includes(cookie) ? cookie : negotiatedLocale(request.headers.get("accept-language"));
  const headers = new Headers({
    location: "/" + locale + "/",
    "cache-control": "private, no-store",
    vary: "Cookie, Accept-Language",
  });
  return new Response(null, { status: 302, headers });
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (url.pathname === "/") return rootResponse(request);
    if (url.pathname === "/__locale") return localeResponse(request);
    if (env && env.ASSETS && env.ASSETS.fetch) return env.ASSETS.fetch(request);
    return new Response("not found\n", { status: 404 });
  },
};
`
