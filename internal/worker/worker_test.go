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
const env = { ASSETS: { async fetch() { return new Response("asset", { status: 200 }); } } };
const cases = [
  ["root-default", "/", {}, 302, "/ja/"],
  ["root-cookie", "/", {"cookie":"cfgb_locale=en","accept-language":"ja"}, 302, "/en/"],
  ["root-quality", "/", {"accept-language":"ja;q=0.2, en-US;q=0.9"}, 302, "/en/"],
  ["root-zero", "/", {"accept-language":"en;q=0, ja;q=0.5"}, 302, "/ja/"],
  ["root-unsupported", "/", {"cookie":"cfgb_locale=fr","accept-language":"fr"}, 302, "/ja/"],
  ["locale-next", "/__locale?lang=en&next=%2Fen%2Fabout%2F", {}, 303, "/en/about/"],
  ["locale-external", "/__locale?lang=en&next=https%3A%2F%2Fevil.invalid%2F", {}, 303, "/en/"],
  ["locale-network", "/__locale?lang=en&next=%2F%2Fevil.invalid%2F", {}, 303, "/en/"],
  ["locale-wrong", "/__locale?lang=en&next=%2Fja%2Fabout%2F", {}, 303, "/en/"],
];
for (const [id, path, headers, status, location] of cases) {
  const response = await worker.fetch(new Request("https://example.invalid" + path, { headers }), env);
  if (response.status !== status) throw new Error(id + " status " + response.status);
  if (response.headers.get("location") !== location) throw new Error(id + " location " + response.headers.get("location"));
  const cache = response.headers.get("cache-control") || "";
  if (!cache.includes("no-store")) throw new Error(id + " cache " + cache);
  if (path === "/" && response.headers.get("vary") !== "Cookie, Accept-Language") throw new Error(id + " vary");
}
const set = await worker.fetch(new Request("https://example.invalid/__locale?lang=en&next=%2Fen%2F"), env);
if (!set.headers.get("set-cookie").includes("cfgb_locale=en")) throw new Error("cookie");
if (!set.headers.get("set-cookie").includes("HttpOnly")) throw new Error("httponly");
const asset = await worker.fetch(new Request("https://example.invalid/ja/posts/x/"), env);
if (asset.status !== 200 || await asset.text() !== "asset") throw new Error("asset delegate");
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
