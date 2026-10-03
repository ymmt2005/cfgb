// Package frontmatter isolates Markdown front matter and decodes it with goccy/go-yaml.
package frontmatter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Post is one localized article after front matter has been removed.
type Post struct {
	ID         string         `json:"id"`
	File       string         `json:"file"`
	Body       string         `json:"body"`
	Group      string         `json:"group"`
	Year       string         `json:"year"`
	ArticleKey string         `json:"articleKey"`
	Locale     string         `json:"locale"`
	Data       map[string]any `json:"data"`
}

// Prose is a home, about, or aside file. v1 keeps the whole file as the body.
type Prose struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Locale string `json:"locale"`
	File   string `json:"file"`
	Body   string `json:"body"`
}

// Index is the normalized metadata passed to the renderer.
type Index struct {
	Topics map[string]map[string]string `json:"topics"`
	Posts  []Post                       `json:"posts"`
	Prose  []Prose                      `json:"prose"`
}

// Split separates an opening front-matter block from the Markdown body.
// The closing delimiter is a complete unindented "---" line. Indented text,
// including an indented "---", stays inside the block. Without an opening
// delimiter the entire input is the body.
func Split(r io.Reader) (front []byte, body []byte, found bool, err error) {
	br := bufio.NewReader(r)
	if err := rejectBOM(br); err != nil {
		return nil, nil, false, err
	}
	content, raw, err := readLine(br)
	if err == io.EOF && len(raw) == 0 {
		return nil, nil, false, nil
	}
	if err != nil && err != io.EOF {
		return nil, nil, false, err
	}
	if content != "---" {
		rest, readErr := io.ReadAll(br)
		if readErr != nil {
			return nil, nil, false, readErr
		}
		return nil, append(raw, rest...), false, nil
	}
	var block bytes.Buffer
	for {
		line, lineRaw, lineErr := readLine(br)
		if lineErr == io.EOF && len(lineRaw) == 0 {
			return nil, nil, true, fmt.Errorf("unterminated front matter")
		}
		if lineErr != nil && lineErr != io.EOF {
			return nil, nil, true, lineErr
		}
		if line == "---" {
			rest, readErr := io.ReadAll(br)
			if readErr != nil {
				return nil, nil, true, readErr
			}
			return block.Bytes(), rest, true, nil
		}
		block.Write(lineRaw)
		if lineErr == io.EOF {
			return nil, nil, true, fmt.Errorf("unterminated front matter")
		}
	}
}

// Document decodes one YAML document. Timestamps and other scalars keep the
// types produced for a generic value, and strings stay strings.
func Document(raw []byte) (any, error) {
	if bytes.HasPrefix(raw, utf8BOM) {
		return nil, fmt.Errorf("UTF-8 BOM is not allowed")
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var doc any
	if err := dec.Decode(&doc); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("YAML document is empty")
		}
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("YAML document must be a single document")
	} else if err != io.EOF {
		return nil, err
	}
	return doc, nil
}

// ArticleData converts a decoded mapping into article metadata.
// A string where an array is required is an error. Missing values are not
// replaced with empty strings or empty arrays.
func ArticleData(doc any) (map[string]any, error) {
	mapping, ok := doc.(map[string]any)
	if !ok || mapping == nil {
		return nil, fmt.Errorf("front matter must be a mapping")
	}
	data := map[string]any{}
	for _, key := range []string{"title", "slug", "publishedAt"} {
		text, err := requiredString(mapping, key)
		if err != nil {
			return nil, err
		}
		data[key] = text
	}
	topics, err := requiredStrings(mapping, "topics")
	if err != nil {
		return nil, err
	}
	data["topics"] = topics
	for _, key := range []string{"updatedAt", "summary", "ogImage"} {
		text, present, err := optionalString(mapping, key)
		if err != nil {
			return nil, err
		}
		if present {
			data[key] = text
		}
	}
	aliases, present, err := optionalStrings(mapping, "aliases")
	if err != nil {
		return nil, err
	}
	if present {
		data["aliases"] = aliases
	}
	return data, nil
}

// ReadArticle splits a required article and decodes only its front-matter bytes.
func ReadArticle(r io.Reader) (map[string]any, []byte, error) {
	front, body, found, err := Split(r)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("article front matter is required")
	}
	doc, err := Document(front)
	if err != nil {
		return nil, nil, err
	}
	data, err := ArticleData(doc)
	if err != nil {
		return nil, nil, err
	}
	return data, body, nil
}

// Topics decodes a topics.yaml document. Values that are not locale mappings
// are errors.
func Topics(raw []byte) (map[string]map[string]string, error) {
	doc, err := Document(raw)
	if err != nil {
		return nil, err
	}
	mapping, ok := doc.(map[string]any)
	if !ok || mapping == nil {
		return nil, fmt.Errorf("topics.yaml must be a mapping")
	}
	topics := make(map[string]map[string]string, len(mapping))
	for key, value := range mapping {
		labels, ok := value.(map[string]any)
		if !ok || labels == nil {
			return nil, fmt.Errorf("topic %s must be a mapping", key)
		}
		converted := make(map[string]string, len(labels))
		for locale, label := range labels {
			text, ok := label.(string)
			if !ok {
				return nil, fmt.Errorf("topic %s label %s must be a string", key, locale)
			}
			converted[locale] = text
		}
		topics[key] = converted
	}
	return topics, nil
}

// Collect reads staged articles, prose, and topics.yaml.
func Collect(contentRoot, topicsFile string, locales []string) (Index, error) {
	var index Index
	rawTopics, err := os.ReadFile(topicsFile)
	if err != nil {
		return index, err
	}
	topics, err := Topics(rawTopics)
	if err != nil {
		return index, fmt.Errorf("topics.yaml: %w", err)
	}
	index.Topics = topics
	root, err := os.OpenRoot(contentRoot)
	if err != nil {
		return index, err
	}
	defer root.Close()
	posts, err := collectPosts(contentRoot, root, locales)
	if err != nil {
		return index, err
	}
	prose, err := collectProse(contentRoot, root, locales)
	if err != nil {
		return index, err
	}
	index.Posts = posts
	index.Prose = prose
	if index.Topics == nil {
		index.Topics = map[string]map[string]string{}
	}
	if index.Posts == nil {
		index.Posts = []Post{}
	}
	if index.Prose == nil {
		index.Prose = []Prose{}
	}
	return index, nil
}

func collectPosts(contentRoot string, root *os.Root, locales []string) ([]Post, error) {
	postsRoot, err := root.OpenRoot("posts")
	if err != nil {
		if os.IsNotExist(err) {
			return []Post{}, nil
		}
		return nil, err
	}
	defer postsRoot.Close()
	years, err := readDir(postsRoot, ".")
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, locale := range locales {
		enabled[locale] = true
	}
	var posts []Post
	for _, year := range years {
		if !year.IsDir() || !isYear(year.Name()) {
			continue
		}
		keys, err := readDir(postsRoot, year.Name())
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if !key.IsDir() {
				continue
			}
			group, err := postsRoot.OpenRoot(year.Name() + "/" + key.Name())
			if err != nil {
				return nil, err
			}
			err = readGroup(group, contentRoot, year.Name(), key.Name(), enabled, &posts)
			group.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	seen := map[string]string{}
	for _, post := range posts {
		slug, _ := post.Data["slug"].(string)
		id := post.Locale + ":" + slug
		if previous, ok := seen[id]; ok {
			return nil, fmt.Errorf("duplicate slug %s in %s (%s and %s)", slug, post.Locale, previous, post.ID)
		}
		seen[id] = post.ID
	}
	sort.Slice(posts, func(i, j int) bool {
		left, lok := time.Parse(time.RFC3339, posts[i].Data["publishedAt"].(string))
		right, rok := time.Parse(time.RFC3339, posts[j].Data["publishedAt"].(string))
		if lok == nil && rok == nil && !left.Equal(right) {
			return left.After(right)
		}
		if posts[i].ArticleKey != posts[j].ArticleKey {
			return posts[i].ArticleKey < posts[j].ArticleKey
		}
		return posts[i].Locale < posts[j].Locale
	})
	return posts, nil
}

func readGroup(group *os.Root, contentRoot, year, articleKey string, locales map[string]bool, posts *[]Post) error {
	entries, err := readDir(group, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		locale := strings.TrimSuffix(name, ".md")
		if !locales[locale] {
			continue
		}
		file, err := group.Open(name)
		if err != nil {
			return err
		}
		data, body, err := ReadArticle(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("posts/%s/%s/%s: %w", year, articleKey, name, err)
		}
		*posts = append(*posts, Post{
			ID:         "posts/" + year + "/" + articleKey + "/" + locale,
			File:       filepath.Join(contentRoot, "posts", year, articleKey, name),
			Body:       string(body),
			Group:      year + "/" + articleKey,
			Year:       year,
			ArticleKey: articleKey,
			Locale:     locale,
			Data:       data,
		})
	}
	return nil
}

func collectProse(contentRoot string, root *os.Root, locales []string) ([]Prose, error) {
	specs := []struct {
		kind string
		dir  string
		id   string
	}{
		{kind: "home", dir: "home", id: "home"},
		{kind: "about", dir: "pages/about", id: "pages/about"},
		{kind: "aside", dir: "aside", id: "aside"},
	}
	var prose []Prose
	for _, spec := range specs {
		for _, locale := range locales {
			name := spec.dir + "/" + locale + ".md"
			file, err := root.Open(name)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			body, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if bytes.HasPrefix(body, utf8BOM) {
				return nil, fmt.Errorf("%s: UTF-8 BOM is not allowed", name)
			}
			prose = append(prose, Prose{
				ID:     spec.id + "/" + locale,
				Kind:   spec.kind,
				Locale: locale,
				File:   filepath.Join(contentRoot, filepath.FromSlash(name)),
				Body:   string(body),
			})
		}
	}
	sort.Slice(prose, func(i, j int) bool { return prose[i].ID < prose[j].ID })
	if prose == nil {
		prose = []Prose{}
	}
	return prose, nil
}

func requiredString(doc map[string]any, key string) (string, error) {
	text, present, err := optionalString(doc, key)
	if err != nil {
		return "", err
	}
	if !present || text == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return text, nil
}

func optionalString(doc map[string]any, key string) (string, bool, error) {
	value, ok := doc[key]
	if !ok {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("%s must be a string", key)
	}
	return text, true, nil
}

func requiredStrings(doc map[string]any, key string) ([]string, error) {
	values, present, err := optionalStrings(doc, key)
	if err != nil {
		return nil, err
	}
	if !present || len(values) == 0 {
		return nil, fmt.Errorf("%s must be a nonempty array of strings", key)
	}
	return values, nil
}

func optionalStrings(doc map[string]any, key string) ([]string, bool, error) {
	value, ok := doc[key]
	if !ok {
		return nil, false, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false, fmt.Errorf("%s must be an array of strings", key)
	}
	values := make([]string, len(items))
	for i, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, false, fmt.Errorf("%s must be an array of strings", key)
		}
		values[i] = text
	}
	return values, true, nil
}

func rejectBOM(br *bufio.Reader) error {
	prefix, err := br.Peek(len(utf8BOM))
	if err != nil && err != io.EOF {
		return err
	}
	if bytes.HasPrefix(prefix, utf8BOM) {
		return fmt.Errorf("UTF-8 BOM is not allowed")
	}
	return nil
}

func readLine(br *bufio.Reader) (string, []byte, error) {
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", nil, err
	}
	if err == io.EOF && line == "" {
		return "", nil, io.EOF
	}
	raw := []byte(line)
	content := strings.TrimSuffix(line, "\n")
	content = strings.TrimSuffix(content, "\r")
	if err == io.EOF {
		return content, raw, io.EOF
	}
	return content, raw, nil
}

func readDir(root *os.Root, name string) ([]os.DirEntry, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.ReadDir(-1)
}

func isYear(name string) bool {
	if len(name) != 4 {
		return false
	}
	for _, char := range name {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
