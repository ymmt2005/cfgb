// Package frontmatter isolates Markdown front matter and decodes it with goccy/go-yaml.
package frontmatter

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/ymmt2005/cfgb/internal/yamlutil"
	"github.com/ymmt2005/cfgb/schemas"
)

// ValidationError identifies invalid source content without disguising I/O failures.
// Callers can recover its diagnostic code through wrapped file context.
type ValidationError struct {
	Code string
	Err  error
}

func (e *ValidationError) Error() string { return e.Code + ": " + e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

func invalid(format string, args ...any) error {
	return &ValidationError{Code: "E_SCHEMA", Err: fmt.Errorf(format, args...)}
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}
var articleKeyPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

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
			return nil, nil, true, invalid("unterminated front matter")
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
		// bytes.Buffer.Write is documented to always return a nil error.
		block.Write(lineRaw)
		if lineErr == io.EOF {
			return nil, nil, true, invalid("unterminated front matter")
		}
	}
}

// Document decodes one YAML document. Timestamps and other scalars keep the
// types produced for a generic value, and strings stay strings.
func Document(raw []byte) (any, error) {
	if !utf8.Valid(raw) {
		return nil, invalid("YAML must be valid UTF-8")
	}
	if bytes.HasPrefix(raw, utf8BOM) {
		return nil, invalid("UTF-8 BOM is not allowed")
	}
	parsed, err := parser.ParseBytes(raw, 0)
	if err != nil {
		return nil, invalid("YAML: %w", err)
	}
	if len(parsed.Docs) == 0 || len(parsed.Docs) == 1 && parsed.Docs[0].Body == nil {
		return nil, invalid("YAML document is empty")
	}
	if len(parsed.Docs) != 1 {
		return nil, invalid("YAML document must be a single document")
	}
	if err := yamlutil.ValidateTags(parsed.Docs[0].Body); err != nil {
		return nil, invalid("YAML: %w", err)
	}
	var doc any
	if err := yaml.NodeToValue(parsed.Docs[0].Body, &doc); err != nil {
		return nil, invalid("YAML: %w", err)
	}
	return doc, nil
}

// ArticleData converts a decoded mapping into article metadata.
// A string where an array is required is an error. Missing values are not
// replaced with empty strings or empty arrays.
func ArticleData(doc any) (map[string]any, error) {
	mapping, ok := doc.(map[string]any)
	if !ok || mapping == nil {
		return nil, invalid("front matter must be a mapping")
	}
	if err := schemas.ValidateArticle(mapping); err != nil {
		return nil, invalid("article schema: %w", err)
	}
	// Schema validation happens before projection so fields cannot disappear
	// silently. Keep the array representation expected by Go consumers.
	data := make(map[string]any, len(mapping))
	for key, value := range mapping {
		if key == "topics" || key == "aliases" {
			if strings, ok := value.([]string); ok {
				data[key] = append([]string{}, strings...)
			} else {
				items := value.([]any)
				strings := make([]string, len(items))
				for i, item := range items {
					strings[i] = item.(string)
				}
				data[key] = strings
			}
		} else {
			data[key] = value
		}
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
		return nil, nil, invalid("article front matter is required")
	}
	if !utf8.Valid(body) {
		return nil, nil, invalid("Markdown body must be valid UTF-8")
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
		return nil, invalid("topics.yaml must be a mapping")
	}
	topics := make(map[string]map[string]string, len(mapping))
	for key, value := range mapping {
		labels, ok := value.(map[string]any)
		if !ok || labels == nil {
			return nil, invalid("topic %s must be a mapping", key)
		}
		converted := make(map[string]string, len(labels))
		for locale, label := range labels {
			text, ok := label.(string)
			if !ok {
				return nil, invalid("topic %s label %s must be a string", key, locale)
			}
			converted[locale] = text
		}
		topics[key] = converted
	}
	return topics, nil
}

// Collect reads staged articles, prose, and topics.yaml.
func Collect(contentRoot, topicsFile string, locales []string) (index Index, err error) {
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
	defer func() { err = errors.Join(err, root.Close()) }()
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

func collectPosts(contentRoot string, root *os.Root, locales []string) (result []Post, err error) {
	postsRoot, err := root.OpenRoot("posts")
	if err != nil {
		if os.IsNotExist(err) {
			return []Post{}, nil
		}
		return nil, err
	}
	defer func() { err = errors.Join(err, postsRoot.Close()) }()
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
			if !articleKeyPattern.MatchString(key.Name()) {
				return nil, &ValidationError{Code: "E_TRANSLATION_GROUP", Err: fmt.Errorf("posts/%s/%s: invalid article key; expected [a-z0-9]+(-[a-z0-9]+)*", year.Name(), key.Name())}
			}
			group, err := postsRoot.OpenRoot(year.Name() + "/" + key.Name())
			if err != nil {
				return nil, err
			}
			err = readGroup(group, contentRoot, year.Name(), key.Name(), enabled, &posts)
			err = errors.Join(err, group.Close())
			if err != nil {
				return nil, err
			}
		}
	}
	seen := map[string]string{}
	published := make(map[string]time.Time, len(posts))
	for _, post := range posts {
		slug := post.Data["slug"].(string) // Required string after article schema validation.
		id := post.Locale + ":" + slug
		if previous, ok := seen[id]; ok {
			return nil, &ValidationError{Code: "E_SLUG_DUPLICATE", Err: fmt.Errorf("duplicate slug %s in %s (%s and %s)", slug, post.Locale, previous, post.ID)}
		}
		seen[id] = post.ID
		date, err := time.Parse(time.RFC3339, post.Data["publishedAt"].(string))
		if err != nil {
			return nil, invalid("%s publishedAt: %w", post.ID, err)
		}
		published[post.ID] = date
	}
	sort.Slice(posts, func(i, j int) bool {
		left, right := published[posts[i].ID], published[posts[j].ID]
		if !left.Equal(right) {
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
		if entry.IsDir() {
			if name != "assets" {
				return &ValidationError{Code: "E_TRANSLATION_GROUP", Err: fmt.Errorf("posts/%s/%s/%s: unexpected directory; only shared assets/ is allowed", year, articleKey, name)}
			}
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		locale := strings.TrimSuffix(name, ".md")
		if !locales[locale] {
			return &ValidationError{Code: "E_TRANSLATION_GROUP", Err: fmt.Errorf("posts/%s/%s/%s: locale %q is not enabled", year, articleKey, name, locale)}
		}
		file, err := group.Open(name)
		if err != nil {
			return err
		}
		data, body, err := ReadArticle(file)
		err = errors.Join(err, file.Close())
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
			err = errors.Join(err, file.Close())
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if !utf8.Valid(body) {
				return nil, invalid("%s: Markdown must be valid UTF-8", name)
			}
			if bytes.HasPrefix(body, utf8BOM) {
				return nil, invalid("%s: UTF-8 BOM is not allowed", name)
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

func rejectBOM(br *bufio.Reader) error {
	prefix, err := br.Peek(len(utf8BOM))
	if err != nil && err != io.EOF {
		return err
	}
	if bytes.HasPrefix(prefix, utf8BOM) {
		return invalid("UTF-8 BOM is not allowed")
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

func readDir(root *os.Root, name string) (entries []os.DirEntry, err error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
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
