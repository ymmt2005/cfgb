package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorkerHostingPrefix(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	source, err := Source(Options{BasePath: "/blog", DefaultLocale: "ja", Locales: []string{"ja", "en"}, Routes: []string{"/ja/", "/en/", "/en/about/"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "worker.mjs"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `import worker from "./worker.mjs";
 const root = await worker.fetch(new Request("https://example.invalid/blog/", {headers: {"accept-language": "en"}}), {});
 if (root.status !== 302 || root.headers.get("location") !== "/blog/en/") throw new Error("prefixed root");
 const selected = await worker.fetch(new Request("https://example.invalid/blog/__locale?lang=en&next=%2Fblog%2Fen%2Fabout%2F"), {});
 if (selected.headers.get("location") !== "/blog/en/about/" || !selected.headers.get("set-cookie").includes("Path=/blog;")) throw new Error("prefixed selection/cookie");
 const outside = await worker.fetch(new Request("https://example.invalid/blog/__locale?lang=en&next=%2Fen%2Fabout%2F"), {});
 if (outside.headers.get("location") !== "/blog/en/") throw new Error("outside prefix fallback");
 await worker.fetch(new Request("https://example.invalid/blog/en/about/"), { ASSETS: { async fetch(request) {
 if (new URL(request.url).pathname !== "/en/about/") throw new Error("asset mount");
 return new Response("ok");
 } } });`
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
}
