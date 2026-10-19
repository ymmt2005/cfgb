package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func previewConfig(raw []byte) ([]byte, error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	delete(cfg, "routes") // Preview never attaches the production custom domain.
	cfg["preview_urls"] = json.RawMessage("true")
	cfg["no_bundle"] = json.RawMessage("true")
	return json.MarshalIndent(cfg, "", "  ")
}

type previewResource struct {
	ID   string   `json:"id"`
	URLs []string `json:"urls"`
}

func (c *previewClient) publish(t *previewTarget, artifact string, manifest deployManifest, renderer, wrangler, configPath string, opts DeployOptions, run toolCommand, extra []string) (err error) {
	// The disposable upload directory gives this attempt a unique provider tag.
	// Cleanup can find a deployment even if Wrangler fails after the remote upload
	// or its JSON output is incomplete, without deleting earlier branch history.
	tag := filepath.Base(filepath.Dir(configPath))
	verified := false
	defer func() {
		if !verified {
			cleanupClient := *c
			ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), time.Minute)
			defer cancel()
			cleanupClient.ctx = ctx
			if cleanupErr := cleanupClient.cleanup(t, tag); cleanupErr != nil {
				err = errors.Join(err, &ExitError{Code: 3, Err: fmt.Errorf("remove failed preview deployment: %w", cleanupErr)})
			}
		}
	}()
	var output bytes.Buffer
	args := []string{wrangler, "preview", "--config", configPath, "--name", t.branch, "--json", "--ignore-base-config", "--tag", tag}
	if err := run(renderer, &output, opts.Stderr, extra, "node", args...); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("cloudflare preview: %w", err)}
	}
	var result struct {
		Preview    previewResource `json:"preview"`
		Deployment previewResource `json:"deployment"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("decode Wrangler preview result: %w", err)}
	}
	if result.Preview.ID == "" || result.Deployment.ID == "" {
		return &ExitError{Code: 3, Err: fmt.Errorf("wrangler omitted preview/deployment identifiers")}
	}
	urls := append(slices.Clone(result.Preview.URLs), result.Deployment.URLs...)
	if len(result.Preview.URLs) == 0 || len(result.Deployment.URLs) == 0 {
		return &ExitError{Code: 3, Err: fmt.Errorf("E_PREVIEW_ACCESS: provider returned no active branch or deployment URLs; configure Preview URL routing")}
	}
	if err := c.verify(t, artifact, manifest, urls); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("E_PREVIEW_ACCESS: post-upload verification: %w", err)}
	}
	verified = true
	for _, address := range slices.Compact(urls) {
		if _, err := fmt.Fprintf(opts.Stdout, "Private preview ready: %s\n", address); err != nil {
			return &ExitError{Code: 3, Err: fmt.Errorf("write preview result: %w", err)}
		}
	}
	return nil
}

func (c *previewClient) cleanup(t *previewTarget, tag string) error {
	path := t.workerPath + "/previews/" + url.PathEscape(t.branch)
	var preview previewResource
	if _, err := c.api("GET", path, t.uploadToken, nil, &preview); err != nil {
		var apiErr *cfAPIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil
		}
		return err
	}
	deployments, err := cfList[struct {
		ID          string            `json:"id"`
		Annotations map[string]string `json:"annotations"`
	}](c, path+"/deployments", t.uploadToken)
	if err != nil {
		return err
	}
	var result error
	for _, deployment := range deployments {
		if deployment.Annotations["workers/tag"] != tag {
			continue
		}
		if deployment.ID == "" {
			result = errors.Join(result, fmt.Errorf("provider omitted failed deployment ID"))
			continue
		}
		if _, err := c.api("DELETE", path+"/deployments/"+url.PathEscape(deployment.ID), t.uploadToken, nil, nil); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

type previewProbe struct {
	path string
	hash [sha256.Size]byte
}

// Probe the supplied bytes, rather than accepting a login/error page with HTTP
// 200. Check generated HTML, media, feeds and the actual Pagefind index files.
func previewProbes(artifact, baseURL string) (probes []previewProbe, err error) {
	prefix, err := siteBasePath(baseURL)
	if err != nil {
		return nil, err
	}
	prefix, err = url.PathUnescape(prefix)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Join(artifact, "site"))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if name == "_headers" || name == "_redirects" || filepath.Base(name) == "404.html" {
			return nil
		}
		raw, err := root.ReadFile(name)
		if err != nil {
			return err
		}
		path := "/" + name
		if strings.HasSuffix(path, "/index.html") {
			path = strings.TrimSuffix(path, "index.html")
		} else {
			path = strings.TrimSuffix(path, ".html")
		}
		probes = append(probes, previewProbe{path: prefix + path, hash: sha256.Sum256(raw)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(probes) == 0 {
		return nil, fmt.Errorf("no published assets available for readiness checks")
	}
	return probes, nil
}

func (c *previewClient) verify(t *previewTarget, artifact string, manifest deployManifest, addresses []string) error {
	probes, err := previewProbes(artifact, manifest.BaseURL)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, address := range addresses {
		origin, err := url.Parse(address)
		if err != nil {
			return fmt.Errorf("parse preview URL: %w", err)
		}
		if origin.Scheme != "https" || origin.User != nil || origin.Port() != "" || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
			return fmt.Errorf("provider returned an unverifiable preview origin")
		}
		matched := false
		for _, host := range t.hosts {
			if globOverlap(strings.ToLower(origin.Hostname()), strings.ToLower(host)) {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("provider returned a preview host outside verified Worker destinations")
		}
		if seen[origin.Host] {
			continue
		}
		seen[origin.Host] = true
		for _, probe := range probes {
			address := (&url.URL{Scheme: origin.Scheme, Host: origin.Host, Path: probe.path}).String()
			if err := c.probe(t, address, probe.hash); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *previewClient) probe(t *previewTarget, address string, expected [sha256.Size]byte) (err error) {
	for _, authorized := range []bool{false, true} {
		if err := func() (err error) {
			req, err := http.NewRequestWithContext(c.ctx, "GET", address, nil)
			if err != nil {
				return err
			}
			if authorized {
				req.Header.Set("CF-Access-Client-Id", t.accessID)
				req.Header.Set("CF-Access-Client-Secret", t.accessSecret)
			}
			resp, err := c.http.Do(req)
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, resp.Body.Close()) }()
			if !authorized {
				denied := resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden
				if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusTemporaryRedirect {
					login, err := url.Parse(resp.Header.Get("Location"))
					if err != nil {
						return err
					}
					denied = login.Scheme == "https" && strings.HasSuffix(login.Hostname(), ".cloudflareaccess.com") && strings.HasPrefix(login.Path, "/cdn-cgi/access/login")
				}
				if !denied {
					return fmt.Errorf("anonymous request was not denied by Access: %s (HTTP %d)", address, resp.StatusCode)
				}
				return nil
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("authorized request failed: %s (HTTP %d)", address, resp.StatusCode)
			}
			hash := sha256.New()
			if _, err := io.Copy(hash, resp.Body); err != nil {
				return fmt.Errorf("read authorized response: %w", err)
			}
			if !bytes.Equal(hash.Sum(nil), expected[:]) {
				return fmt.Errorf("authorized response differs from supplied artifact: %s", address)
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}
