package build

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Corrupt the actual completed artifact, restoring each file before the next
// case. These checks must reject errors that substring smoke assertions miss.
func proveCorpusChecks(t *testing.T, want corpusExpectations, out string) {
	t.Helper()
	sitemap := want.Sitemap.SitemapPaths[0]
	feed := "/" + want.Locales[0] + "/feed.xml"
	article := want.Articles[0].URL + "index.html"
	checkSitemap := func() error { return checkCorpusSitemap(want) }
	checkRSS := func() error { return checkCorpusRSS(want) }
	checkMetadata := func() error { return checkCorpusArticleMetadata(want, out) }
	for _, mutation := range []struct {
		name, route, diagnostic string
		edit                    func([]byte) ([]byte, error)
		check                   func() error
	}{
		{"duplicate sitemap URL", sitemap, "duplicate sitemap URL", editSitemap(func(doc *corpusSitemap) { doc.URLs = append(doc.URLs, doc.URLs[0]) }), checkSitemap},
		{"missing sitemap URL", sitemap, "canonical membership", editSitemap(func(doc *corpusSitemap) { doc.URLs = doc.URLs[1:] }), checkSitemap},
		{"unexpected sitemap URL", sitemap, "missing canonical URL", editSitemap(func(doc *corpusSitemap) { doc.URLs[0].Location = want.absolute("/unexpected/") }), checkSitemap},
		{"wrong sitemap alternate", sitemap, "sitemap alternates", editSitemap(func(doc *corpusSitemap) {
			for i := range doc.URLs {
				if len(doc.URLs[i].Links) > 0 {
					doc.URLs[i].Links[0].Href = "https://preview.invalid/wrong/"
					return
				}
			}
		}), checkSitemap},
		{"wrong sitemap lastmod", sitemap, "sitemap lastmod", editSitemap(func(doc *corpusSitemap) {
			for i := range doc.URLs {
				if doc.URLs[i].Lastmod != "" {
					doc.URLs[i].Lastmod = "2000-01-01T00:00:00Z"
					return
				}
			}
		}), checkSitemap},
		{"missing referenced sitemap", sitemap, "", nil, checkSitemap},
		{"wrong index origin", want.Sitemap.IndexPath, "missing reference", replaceCorpusText(want.absolute(sitemap), "https://preview.invalid"+sitemap), checkSitemap},
		{"duplicate index reference", want.Sitemap.IndexPath, "duplicate sitemap index reference", replaceCorpusText("</sitemapindex>", "<sitemap><loc>"+want.absolute(sitemap)+"</loc></sitemap></sitemapindex>"), checkSitemap},
		{"trailing invalid XML", sitemap, "text after XML root", func(raw []byte) ([]byte, error) { return append(raw, []byte("not XML")...), nil }, checkSitemap},
		{"second XML root", feed, "multiple XML roots", func(raw []byte) ([]byte, error) { return append(raw, []byte("<rss/>")...), nil }, checkRSS},
		{"missing RSS item", feed, "RSS membership", editRSS(func(doc *corpusRSS) { doc.Channel.Items = doc.Channel.Items[1:] }), checkRSS},
		{"duplicate RSS item", feed, "RSS membership", editRSS(func(doc *corpusRSS) { doc.Channel.Items = append(doc.Channel.Items, doc.Channel.Items[0]) }), checkRSS},
		{"wrong RSS order", feed, "membership/order", editRSS(func(doc *corpusRSS) {
			doc.Channel.Items[0], doc.Channel.Items[1] = doc.Channel.Items[1], doc.Channel.Items[0]
		}), checkRSS},
		{"wrong RSS summary", feed, "title/summary", editRSS(func(doc *corpusRSS) { doc.Channel.Items[0].Description = "wrong & summary" }), checkRSS},
		{"wrong RSS publication date", feed, "publication date", editRSS(func(doc *corpusRSS) { doc.Channel.Items[0].Published = "Sat, 01 Jan 2000 00:00:00 GMT" }), checkRSS},
		{"wrong RSS locale", feed, "membership/order", editRSS(func(doc *corpusRSS) {
			doc.Channel.Items[0].Link = want.absolute("/" + want.Locales[1] + "/posts/wrong/")
		}), checkRSS},
		{"wrong RSS GUID", feed, "membership/order", editRSS(func(doc *corpusRSS) { doc.Channel.Items[0].GUID = "https://preview.invalid/wrong/" }), checkRSS},
		{"wrong canonical", article, "AssertionError", replaceCorpusText(want.absolute(want.Articles[0].URL), "https://preview.invalid/wrong/"), checkMetadata},
		{"wrong OG title", article, "AssertionError", replaceCorpusText(`property="og:title" content="`, `property="og:title" content="WRONG `), checkMetadata},
		{"wrong OG locale", article, "catalog OG locale", replaceCorpusText(`property="og:locale" content="`, `property="og:locale" content="WRONG_`), checkMetadata},
		{"missing sharing description", article, "expected one meta", replaceCorpusText(`property="og:description"`, `property="og:wrong-description"`), checkMetadata},
		{"wrong image dimensions", article, "AssertionError", replaceCorpusText(`property="og:image:width" content="`, `property="og:image:width" content="0`), checkMetadata},
		{"wrong JSON-LD headline", article, "AssertionError", replaceCorpusText(`"headline":"`, `"headline":"WRONG `), checkMetadata},
		{"invalid JSON-LD", article, "SyntaxError", replaceCorpusText(`"@context":`, `"@context"=`), checkMetadata},
	} {
		t.Run("reject "+mutation.name, func(t *testing.T) {
			file := want.file(mutation.route)
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(file, raw, 0o644); err != nil {
					t.Error(err)
				}
			})
			if mutation.edit == nil {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			} else {
				changed, err := mutation.edit(raw)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, changed, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err = mutation.check()
			if mutation.edit == nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing file failure lost cause: %v", err)
			}
			if err == nil || !strings.Contains(err.Error(), mutation.diagnostic) {
				t.Fatalf("corruption did not produce %q: %v", mutation.diagnostic, err)
			}
		})
	}
	t.Run("reject swapped fallback image URLs", func(t *testing.T) {
		var articles []corpusArticle
		for _, article := range want.Articles {
			if article.Data.OGImage == "" {
				articles = append(articles, article)
				if len(articles) == 2 {
					break
				}
			}
		}
		if len(articles) != 2 {
			t.Fatal("image ownership corruption needs two fallback articles")
		}
		pattern := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
		var original [2][]byte
		var images [2][]byte
		for i, article := range articles {
			file := want.file(article.URL + "index.html")
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			match := pattern.FindSubmatch(raw)
			if match == nil {
				t.Fatalf("OG image missing from %s", article.URL)
			}
			original[i], images[i] = raw, match[1]
			t.Cleanup(func() {
				if err := os.WriteFile(file, raw, 0o644); err != nil {
					t.Error(err)
				}
			})
		}
		for i, article := range articles {
			// Keep OG, Twitter and JSON-LD consistent and preserve uniqueness;
			// only the association with source identity is wrong.
			changed := bytes.ReplaceAll(original[i], images[i], images[1-i])
			if err := os.WriteFile(want.file(article.URL+"index.html"), changed, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := checkMetadata(); err == nil || !strings.Contains(err.Error(), "source identity OG image URL") {
			t.Fatalf("swapped image URLs did not fail the ownership assertion: %v", err)
		}
	})
	t.Run("reject unreferenced sitemap", func(t *testing.T) {
		file := filepath.Join(want.SiteRoot, "sitemap-extra.xml")
		if err := os.WriteFile(file, []byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"/>`), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Remove(file); err != nil {
				t.Error(err)
			}
		})
		if err := checkSitemap(); err == nil || !strings.Contains(err.Error(), "unreferenced sitemap file") {
			t.Fatalf("unreferenced sitemap accepted: %v", err)
		}
	})
}

func replaceCorpusText(old, replacement string) func([]byte) ([]byte, error) {
	return func(raw []byte) ([]byte, error) {
		if !bytes.Contains(raw, []byte(old)) {
			return nil, fmt.Errorf("mutation text absent: %s", old)
		}
		return bytes.Replace(raw, []byte(old), []byte(replacement), 1), nil
	}
}

func editSitemap(edit func(*corpusSitemap)) func([]byte) ([]byte, error) {
	return func(raw []byte) ([]byte, error) {
		var doc corpusSitemap
		if err := xml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		edit(&doc)
		return xml.Marshal(doc)
	}
}

func editRSS(edit func(*corpusRSS)) func([]byte) ([]byte, error) {
	return func(raw []byte) ([]byte, error) {
		var doc corpusRSS
		if err := xml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		edit(&doc)
		return xml.Marshal(doc)
	}
}
