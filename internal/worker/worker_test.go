package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocaleWorker(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	source, err := Source(Options{
		DefaultLocale: "ja",
		Locales:       []string{"en", "ja"},
		Routes: []string{
			"/ja/",
			"/en/",
			"/en/about/",
			"/ja/about/",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "worker.js"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
import worker from "./worker.js";
const baseline = {
  "x-content-type-options": "nosniff",
  "referrer-policy": "strict-origin-when-cross-origin",
  "x-frame-options": "DENY",
  "permissions-policy": "camera=(), microphone=(), geolocation=()",
  "content-security-policy": "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self' 'wasm-unsafe-eval'",
};
function checkHeaders(id, response) {
  for (const [header, value] of Object.entries(baseline)) {
    if (response.headers.get(header) !== value) throw new Error(id + " missing/wrong " + header);
  }
}
const assetResponse = new Response("asset", { status: 200, headers: { "cache-control": "public, max-age=42", etag: '"asset"' } });
const env = { ASSETS: { async fetch() { return assetResponse; } } };
const cases = [
  ["root-default", "/", {}, 302, "/ja/"],
  ["root-cookie", "/", {"cookie":"cfgb_locale=en","accept-language":"ja"}, 302, "/en/"],
  ["root-quality", "/", {"accept-language":"ja;q=0.2, en-US;q=0.9"}, 302, "/en/"],
  ["root-zero", "/", {"accept-language":"en;q=0, ja;q=0.5"}, 302, "/ja/"],
  ["root-upper-q", "/", {"accept-language":"en;Q=0, ja;q=0.5"}, 302, "/ja/"],
  ["root-unsupported", "/", {"cookie":"cfgb_locale=fr","accept-language":"fr"}, 302, "/ja/"],
  ["root-cookie-case", "/", {"cookie":"cfgb_locale=EN","accept-language":"ja"}, 302, "/ja/"],
  ["root-cookie-region", "/", {"cookie":"cfgb_locale=en-US","accept-language":"ja"}, 302, "/ja/"],
  ["root-unsupported-region", "/", {"accept-language":"pt-BR"}, 302, "/ja/"],
  ["locale-next", "/__locale?lang=en&next=%2Fen%2Fabout%2F", {}, 303, "/en/about/"],
  ["locale-external", "/__locale?lang=en&next=https%3A%2F%2Fevil.invalid%2F", {}, 303, "/en/"],
  ["locale-network", "/__locale?lang=en&next=%2F%2Fevil.invalid%2F", {}, 303, "/en/"],
  ["locale-wrong", "/__locale?lang=en&next=%2Fja%2Fabout%2F", {}, 303, "/en/"],
];
for (const [id, path, headers, status, location] of cases) {
  const response = await worker.fetch(new Request("https://example.invalid" + path, { headers }), env);
  checkHeaders(id, response);
  if (response.status !== status) throw new Error(id + " status " + response.status);
  if (response.headers.get("location") !== location) throw new Error(id + " location " + response.headers.get("location"));
  const cache = response.headers.get("cache-control") || "";
  if (!cache.includes("no-store")) throw new Error(id + " cache " + cache);
  if (path === "/" && response.headers.get("vary") !== "Cookie, Accept-Language") throw new Error(id + " vary");
}
const set = await worker.fetch(new Request("https://example.invalid/__locale?lang=en&next=%2Fen%2F"), env);
const cookie = set.headers.get("set-cookie") || "";
for (const part of ["cfgb_locale=en", "Secure", "HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=31536000"]) {
  if (!cookie.includes(part)) throw new Error("cookie " + part + " in " + cookie);
}
for (const [id, path, method, status, body, cache, allow] of [
  ["root-head", "/", "HEAD", 302, "", "private, no-store", null],
  ["root-method", "/", "POST", 405, "method not allowed\n", "private, no-store", "GET, HEAD"],
  ["locale-method", "/__locale?lang=en", "POST", 405, "method not allowed\n", "no-store", "GET"],
  ["locale-head", "/__locale?lang=en", "HEAD", 405, "method not allowed\n", "no-store", "GET"],
  ["locale-invalid", "/__locale?lang=fr", "GET", 400, "unsupported locale\n", "no-store", null],
  ["locale-case", "/__locale?lang=EN", "GET", 400, "unsupported locale\n", "no-store", null],
  ["locale-region", "/__locale?lang=en-US", "GET", 400, "unsupported locale\n", "no-store", null],
  ["locale-unsupported-region", "/__locale?lang=pt-BR", "GET", 400, "unsupported locale\n", "no-store", null],
  ["locale-missing", "/__locale", "GET", 400, "unsupported locale\n", "no-store", null],
  ["no-assets", "/missing", "GET", 404, "not found\n", null, null],
]) {
  const response = await worker.fetch(new Request("https://example.invalid" + path, { method }), id === "no-assets" ? {} : env);
  checkHeaders(id, response);
  if (response.status !== status || await response.text() !== body) throw new Error(id + " status/body");
  if (response.headers.get("cache-control") !== cache || response.headers.get("allow") !== allow) throw new Error(id + " cache/allow");
  if (id === "root-head" && response.headers.get("location") !== "/ja/") throw new Error(id + " location");
}
const asset = await worker.fetch(new Request("https://example.invalid/ja/posts/x/"), env);
if (asset !== assetResponse || asset.status !== 200 || await asset.text() !== "asset" || asset.headers.get("cache-control") !== "public, max-age=42" || asset.headers.get("etag") !== '"asset"') throw new Error("asset delegate");
console.log("ok");
`
	if err := os.WriteFile(filepath.Join(dir, "harness.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "harness.mjs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
