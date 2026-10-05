package build

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ymmt2005/cfgb/internal/config"
	"github.com/ymmt2005/cfgb/internal/frontmatter"
)

type corpusArticle struct {
	ArticleKey string               `json:"articleKey"`
	Locale     string               `json:"locale"`
	Path       string               `json:"path"`
	URL        string               `json:"url"`
	Year       string               `json:"year"`
	Alternates map[string]string    `json:"alternates"`
	Data       frontmatter.Metadata `json:"data"`
}

type corpusExpectations struct {
	Root     string          `json:"root"`
	SiteRoot string          `json:"siteRoot"`
	BaseURL  string          `json:"baseURL"`
	Title    string          `json:"title"`
	Articles []corpusArticle `json:"articles"`
	Locales  []string        `json:"-"`
	Sitemap  struct {
		IndexPath         string   `json:"indexPath"`
		SitemapPaths      []string `json:"sitemapPaths"`
		CanonicalPaths    []string `json:"canonicalPaths"`
		ExcludedPaths     []string `json:"excludedPaths"`
		ArticleAlternates []struct {
			Path       string            `json:"path"`
			Alternates map[string]string `json:"alternates"`
		} `json:"articleAlternates"`
	} `json:"-"`
}

func loadCorpusExpectations(root, out, baseURL string) (corpusExpectations, error) {
	var want corpusExpectations
	cfg, err := config.Load(root)
	if err != nil {
		return want, err
	}
	location, err := time.LoadLocation(cfg.Site.Timezone)
	if err != nil {
		return want, err
	}
	want.Root, want.SiteRoot, want.Title = root, filepath.Join(out, "site"), cfg.Site.Title
	want.BaseURL = cfg.Site.BaseURL
	if baseURL != "" {
		want.BaseURL = baseURL
	}
	for locale := range cfg.Locales {
		want.Locales = append(want.Locales, locale)
	}
	slices.Sort(want.Locales)
	for name, dest := range map[string]any{"articles.json": &want.Articles, "sitemap.json": &want.Sitemap} {
		raw, err := os.ReadFile(filepath.Join(root, "tests", "expected", name))
		if err != nil {
			return want, err
		}
		if err := json.Unmarshal(raw, dest); err != nil {
			return want, fmt.Errorf("%s: %w", name, err)
		}
	}
	index, err := frontmatter.Collect(filepath.Join(root, cfg.Content.Root), filepath.Join(root, cfg.Content.Topics), want.Locales)
	if err != nil {
		return want, err
	}
	sources := make(map[string]frontmatter.Post)
	for _, post := range index.Posts {
		rel, err := filepath.Rel(root, post.File)
		if err != nil {
			return want, err
		}
		sources[filepath.ToSlash(rel)] = post
	}
	for i := range want.Articles {
		article := &want.Articles[i]
		post, ok := sources[article.Path]
		if !ok || post.Locale != article.Locale || post.Group != article.ArticleKey {
			return want, fmt.Errorf("article fixture does not match source: %s", article.Path)
		}
		article.Data = post.Data
		article.Year = post.Data.PublishedAt.In(location).Format("2006")
		delete(sources, article.Path)
	}
	if len(sources) != 0 {
		missing := make([]string, 0, len(sources))
		for source := range sources {
			missing = append(missing, source)
		}
		slices.Sort(missing)
		return want, fmt.Errorf("article fixtures omit source articles: %v", missing)
	}
	return want, nil
}

func (want corpusExpectations) absolute(route string) string {
	return strings.TrimRight(want.BaseURL, "/") + route
}

func (want corpusExpectations) file(route string) string {
	return filepath.Join(want.SiteRoot, filepath.FromSlash(strings.TrimPrefix(route, "/")))
}

type corpusSitemapLink struct {
	Rel      string `xml:"rel,attr"`
	Language string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

type corpusSitemapURL struct {
	Location string              `xml:"loc"`
	Lastmod  string              `xml:"lastmod"`
	Links    []corpusSitemapLink `xml:"http://www.w3.org/1999/xhtml link"`
}

type corpusSitemap struct {
	XMLName xml.Name           `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []corpusSitemapURL `xml:"url"`
}

type corpusRSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel struct {
		Title       string          `xml:"title"`
		Link        string          `xml:"link"`
		Description string          `xml:"description"`
		Items       []corpusRSSItem `xml:"item"`
	} `xml:"channel"`
}

type corpusRSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	Published   string `xml:"pubDate"`
}

// Read the entire XML document, so trailing malformed XML or a second root
// cannot be hidden by decoding only the first element.
func readCorpusXML(file string, dest any) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		switch token := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return fmt.Errorf("%s: text after XML root", file)
			}
		case xml.StartElement:
			return fmt.Errorf("%s: multiple XML roots", file)
		}
	}
}

func checkCorpusSitemap(want corpusExpectations) error {
	var index struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 sitemapindex"`
		Files   []struct {
			Location string `xml:"loc"`
		} `xml:"sitemap"`
	}
	if err := readCorpusXML(want.file(want.Sitemap.IndexPath), &index); err != nil {
		return err
	}
	files := make(map[string]bool)
	for _, file := range index.Files {
		if files[file.Location] {
			return fmt.Errorf("duplicate sitemap index reference: %s", file.Location)
		}
		files[file.Location] = true
	}
	if len(files) != len(want.Sitemap.SitemapPaths) {
		return fmt.Errorf("sitemap index references %v, want %v", files, want.Sitemap.SitemapPaths)
	}
	all := make(map[string]corpusSitemapURL)
	for _, route := range want.Sitemap.SitemapPaths {
		if !files[want.absolute(route)] {
			return fmt.Errorf("sitemap index missing reference: %s", want.absolute(route))
		}
		var sitemap corpusSitemap
		if err := readCorpusXML(want.file(route), &sitemap); err != nil {
			return err
		}
		for _, item := range sitemap.URLs {
			if _, ok := all[item.Location]; ok {
				return fmt.Errorf("duplicate sitemap URL: %s", item.Location)
			}
			all[item.Location] = item
		}
	}
	entries, err := os.ReadDir(want.SiteRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "sitemap-") && strings.HasSuffix(entry.Name(), ".xml") && "/"+entry.Name() != want.Sitemap.IndexPath && !files[want.absolute("/"+entry.Name())] {
			return fmt.Errorf("unreferenced sitemap file: %s", entry.Name())
		}
	}
	if len(all) != len(want.Sitemap.CanonicalPaths) {
		return fmt.Errorf("sitemap canonical membership differs: got %d URLs, want %d", len(all), len(want.Sitemap.CanonicalPaths))
	}
	for _, route := range want.Sitemap.CanonicalPaths {
		if _, ok := all[want.absolute(route)]; !ok {
			return fmt.Errorf("sitemap missing canonical URL: %s", want.absolute(route))
		}
		if _, err := os.Stat(filepath.Join(want.file(route), "index.html")); err != nil {
			return fmt.Errorf("canonical page %s: %w", route, err)
		}
	}
	for _, route := range want.Sitemap.ExcludedPaths {
		if _, ok := all[want.absolute(route)]; ok {
			return fmt.Errorf("sitemap includes excluded URL: %s", route)
		}
	}
	expectedLinks := make(map[string]map[string]string)
	for _, article := range want.Sitemap.ArticleAlternates {
		expectedLinks[want.absolute(article.Path)] = article.Alternates
	}
	for _, page := range []string{"", "about/"} {
		links := make(map[string]string)
		for _, locale := range want.Locales {
			links[locale] = "/" + locale + "/" + page
		}
		for _, locale := range want.Locales {
			if len(want.Locales) > 1 {
				expectedLinks[want.absolute("/"+locale+"/"+page)] = links
			}
		}
	}
	modified := make(map[string]time.Time)
	for _, article := range want.Articles {
		date := article.Data.PublishedAt
		if article.Data.UpdatedAt != nil {
			date = *article.Data.UpdatedAt
		}
		modified[want.absolute(article.URL)] = date
		if !reflect.DeepEqual(article.Alternates, expectedLinks[want.absolute(article.URL)]) {
			return fmt.Errorf("sitemap and article fixtures disagree: %s", article.URL)
		}
	}
	for location, item := range all {
		links := make(map[string]string)
		for _, link := range item.Links {
			_, duplicate := links[link.Language]
			if link.Rel != "alternate" || duplicate {
				return fmt.Errorf("invalid or duplicate sitemap alternate: %s %+v", location, link)
			}
			links[link.Language] = link.Href
		}
		alternates := make(map[string]string)
		for locale, route := range expectedLinks[location] {
			alternates[locale] = want.absolute(route)
		}
		if !reflect.DeepEqual(links, alternates) {
			return fmt.Errorf("sitemap alternates for %s = %v, want %v", location, links, alternates)
		}
		if date, ok := modified[location]; ok {
			actual, err := time.Parse(time.RFC3339, item.Lastmod)
			if err != nil {
				return fmt.Errorf("sitemap lastmod for %s: %w", location, err)
			}
			if !actual.Equal(date) {
				return fmt.Errorf("sitemap lastmod for %s = %q, want %s", location, item.Lastmod, date.UTC().Format(time.RFC3339))
			}
		} else if item.Lastmod != "" {
			return fmt.Errorf("non-article sitemap URL has lastmod: %s", location)
		}
	}
	robots, err := os.ReadFile(want.file("/robots.txt"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(robots), "Sitemap: "+want.absolute(want.Sitemap.IndexPath)+"\n") {
		return fmt.Errorf("robots.txt does not advertise canonical sitemap index")
	}
	return nil
}

func checkCorpusRSS(want corpusExpectations) error {
	for _, locale := range want.Locales {
		var feed corpusRSS
		if err := readCorpusXML(want.file("/"+locale+"/feed.xml"), &feed); err != nil {
			return err
		}
		if feed.Version != "2.0" || feed.Channel.Title != want.Title || feed.Channel.Description != want.Title || feed.Channel.Link != want.absolute("/"+locale+"/") {
			return fmt.Errorf("%s RSS channel metadata differs: version=%q title=%q description=%q link=%q", locale, feed.Version, feed.Channel.Title, feed.Channel.Description, feed.Channel.Link)
		}
		var articles []corpusArticle
		for _, article := range want.Articles {
			if article.Locale == locale {
				articles = append(articles, article)
			}
		}
		slices.SortFunc(articles, func(a, b corpusArticle) int {
			if comparison := b.Data.PublishedAt.Compare(a.Data.PublishedAt); comparison != 0 {
				return comparison
			}
			return strings.Compare(a.ArticleKey, b.ArticleKey)
		})
		if len(feed.Channel.Items) != len(articles) {
			return fmt.Errorf("%s RSS membership differs: got %d items, want %d", locale, len(feed.Channel.Items), len(articles))
		}
		for i, item := range feed.Channel.Items {
			article := articles[i]
			canonical := want.absolute(article.URL)
			if item.Link != canonical || item.GUID != canonical {
				return fmt.Errorf("%s RSS membership/order at %d: link %q, guid %q, want %q", locale, i, item.Link, item.GUID, canonical)
			}
			if item.Title != article.Data.Title || item.Description != article.Data.Summary {
				return fmt.Errorf("RSS title/summary differs from source: %s", canonical)
			}
			date, err := time.Parse(time.RFC1123, item.Published)
			if err != nil {
				return fmt.Errorf("RSS publication date for %s: %w", canonical, err)
			}
			if !date.Equal(article.Data.PublishedAt) {
				return fmt.Errorf("RSS publication date for %s = %q, want %s", canonical, item.Published, article.Data.PublishedAt)
			}
		}
	}
	return nil
}

func checkCorpusArticleMetadata(want corpusExpectations, out string) error {
	raw, err := os.ReadFile(filepath.Join(out, "build-manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Session string `json:"toolchainSessionId"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("cannot locate corpus metadata helper")
	}
	script := filepath.Join(filepath.Dir(file), "..", "..", "renderer", "tests", "helpers", "corpus-metadata.mjs")
	renderer := filepath.Join(os.TempDir(), manifest.Session, "renderer")
	input, err := json.Marshal(want)
	if err != nil {
		return err
	}
	cmd := exec.Command("node", script, renderer)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("article metadata: %w\n%s", err, output)
	}
	return nil
}

func checkExampleCorpus(t *testing.T, root, out, baseURL string) corpusExpectations {
	t.Helper()
	want, err := loadCorpusExpectations(root, out, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name string
		run  func() error
	}{
		{"sitemap", func() error { return checkCorpusSitemap(want) }},
		{"RSS", func() error { return checkCorpusRSS(want) }},
		{"article metadata", func() error { return checkCorpusArticleMetadata(want, out) }},
	} {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err != nil {
				t.Fatal(err)
			}
		})
	}
	return want
}
