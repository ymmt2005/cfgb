package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type previewHarness struct {
	mu            sync.Mutex
	client        *previewClient
	apps          []accessApp
	policies      []accessPolicy
	policiesByApp map[string][]accessPolicy
	redirects     map[string]struct {
		status   int
		location string
	}
	serviceTokens                                     []map[string]any
	group                                             accessPolicy
	requests                                          []string
	files                                             map[string]string
	failPath, leakPath, wrongPath                     string
	cleanupFail, noURLs, badJSON, uploadFail          bool
	loginRedirect, authorizedRedirect, unexpectedHost bool
	tag                                               string
	uploads                                           int
}

func previewFixture(t *testing.T) (DeployOptions, deployManifest, string, *previewHarness) {
	t.Helper()
	opts, manifest, workspace := deployFixture(t)
	if _, err := output("git", "-C", opts.Dir, "checkout", "-b", "feature/diagram"); err != nil {
		t.Fatal(err)
	}
	manifest.BaseURL = "https://public.invalid/blog%20space"
	manifest.Publications[0].PublishedAt = time.Now().Add(24 * time.Hour)
	writeDeployManifest(t, filepath.Join(opts.Dir, opts.From), manifest)
	h := &previewHarness{
		redirects: map[string]struct {
			status   int
			location string
		}{"/blog space/": {302, "/blog%20space/en/"}, "/blog space": {302, "/blog%20space/en/"}},
		apps:     []accessApp{{ID: "app", Type: "self_hosted", Destinations: []accessDestination{{Type: "preview_worker", WorkerID: "worker-id"}}}},
		policies: []accessPolicy{{Decision: "allow"}, {Decision: "non_identity", Include: []accessRule{{AnyValidServiceToken: &struct{}{}}}}},
		files:    map[string]string{"/blog space/en/": "edited site bytes", "/blog space/image.svg": "<svg/>", "/blog space/en/feed.xml": "<feed/>", "/blog space/pagefind/pagefind.js": "real index script bytes"},
	}
	for path, content := range h.files {
		if path == "/blog space/en/" {
			continue
		}
		file := filepath.Join(opts.Dir, opts.From, "site", strings.TrimPrefix(path, "/blog space/"))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CFGB_CF_ACCESS_API_TOKEN", "read-only-inspection-token")
	t.Setenv("CFGB_CF_ACCESS_CLIENT_ID", "probe-client")
	t.Setenv("CFGB_CF_ACCESS_CLIENT_SECRET", "probe-secret")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.requests = append(h.requests, r.Method+" "+r.Host+r.URL.EscapedPath())
		if r.URL.EscapedPath() == h.failPath {
			http.Error(w, "sensitive-provider-error-body", http.StatusForbidden)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/accounts/") {
			if r.Header.Get("Authorization") != "" {
				t.Error("API credential leaked to preview host")
			}
			authorized := r.Header.Get("CF-Access-Client-Id") == "probe-client" && r.Header.Get("CF-Access-Client-Secret") == "probe-secret"
			if authorized && h.authorizedRedirect {
				w.Header().Set("Location", "https://outside.invalid/login")
				w.WriteHeader(http.StatusFound)
				return
			}
			if !authorized && h.loginRedirect {
				w.Header().Set("Location", "https://team.cloudflareaccess.com/cdn-cgi/access/login/preview")
				w.WriteHeader(http.StatusFound)
				return
			}
			if !authorized && r.URL.Path != h.leakPath {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if redirect, ok := h.redirects[r.URL.Path]; ok {
				w.Header().Set("Location", redirect.location)
				w.WriteHeader(redirect.status)
				return
			}
			data, ok := h.files[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if r.URL.Path == h.wrongPath {
				data = "login page masquerading as success"
			}
			if _, err := io.WriteString(w, data); err != nil {
				t.Error(err)
			}
			return
		}
		if r.Header.Get("CF-Access-Client-Secret") != "" {
			t.Error("probe credential leaked to API")
		}
		wantToken := "Bearer test-token"
		if strings.Contains(r.URL.Path, "/access/") {
			wantToken = "Bearer read-only-inspection-token"
		}
		if r.Header.Get("Authorization") != wantToken {
			t.Errorf("wrong credential for %s", r.URL.Path)
		}
		var result any
		page, pages := 1, 1
		switch {
		case strings.HasSuffix(r.URL.Path, "/workers/workers/example-worker"):
			result = map[string]any{"id": "worker-id", "name": "example-worker", "subdomain": map[string]any{"preview_url_suffix": "-example-worker.team.workers.dev"}, "references": map[string]any{"domains": []map[string]string{{"hostname": "example.com"}}}}
		case strings.HasSuffix(r.URL.Path, "/workers/workers/worker-id") && r.Method == "PATCH":
			var patch struct {
				Subdomain struct{ Enabled, PreviewsEnabled bool } `json:"subdomain"`
			}
			var raw map[string]map[string]bool
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Error(err)
			}
			patch.Subdomain.Enabled = raw["subdomain"]["enabled"]
			patch.Subdomain.PreviewsEnabled = raw["subdomain"]["previews_enabled"]
			if patch.Subdomain.Enabled || !patch.Subdomain.PreviewsEnabled {
				t.Error("unexpected preview URL settings")
			}
			result = map[string]any{}
		case strings.HasSuffix(r.URL.Path, "/access/apps"):
			// Exercise application pagination even for a short first page.
			pages = 2
			if r.URL.Query().Get("page") == "1" {
				result = []accessApp{{ID: "unrelated", Type: "self_hosted", Domain: "public.example.com", SelfHostedDomains: []string{"public.example.com"}}}
			} else {
				result, page = h.apps, 2
			}
		case strings.HasSuffix(r.URL.Path, "/policies"):
			if policies, ok := h.policiesByApp[r.URL.Path]; ok {
				result = policies
			} else {
				result = h.policies
			}
		case strings.HasSuffix(r.URL.Path, "/access/service_tokens"):
			result = h.serviceTokens
		case strings.Contains(r.URL.Path, "/access/groups/"):
			result = h.group
		case strings.HasSuffix(r.URL.Path, "/deployments/failed-deployment") && r.Method == "DELETE":
			if h.cleanupFail {
				http.Error(w, "cleanup error", 500)
				return
			}
			result = map[string]any{}
		case strings.HasSuffix(r.URL.Path, "/deployments"):
			result = []map[string]any{{"id": "earlier-deployment", "annotations": map[string]string{"workers/tag": "previous-success"}}, {"id": "failed-deployment", "annotations": map[string]string{"workers/tag": h.tag}}}
		case strings.Contains(r.URL.Path, "/previews/"):
			result = map[string]string{"id": "preview-id"}
		default:
			t.Errorf("unexpected API request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result, "result_info": map[string]int{"page": page, "total_pages": pages}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client.Transport = transport
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	t.Cleanup(transport.CloseIdleConnections)
	h.client = &previewClient{ctx: context.Background(), apiBase: server.URL, http: client}
	return opts, manifest, workspace, h
}

func (h *previewHarness) run(t *testing.T, manifest deployManifest) toolCommand {
	return func(dir string, stdout, stderr io.Writer, extra []string, name string, args ...string) error {
		t.Helper()
		if slices.Contains(args, "--version") {
			_, err := io.WriteString(stdout, manifest.Toolchain.WranglerVersion)
			return err
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		h.uploads++
		if slices.Contains(args, "--dry-run") {
			if args[1] != "deploy" {
				t.Fatalf("dry run must use Wrangler local validation: %v", args)
			}
			return nil
		}
		if args[1] != "preview" || args[slices.Index(args, "--name")+1] != "feature/diagram" || !slices.Contains(args, "--ignore-base-config") {
			t.Fatalf("preview arguments: %v", args)
		}
		h.tag = args[slices.Index(args, "--tag")+1]
		raw, err := os.ReadFile(args[slices.Index(args, "--config")+1])
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Routes      json.RawMessage
			PreviewURLs bool `json:"preview_urls"`
			NoBundle    bool `json:"no_bundle"`
			Assets      struct{ Directory string }
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Routes != nil || !cfg.PreviewURLs || !cfg.NoBundle {
			t.Fatalf("preview config: %s", raw)
		}
		if raw, err := os.ReadFile(filepath.Join(cfg.Assets.Directory, "blog space", "en", "index.html")); err != nil || string(raw) != "edited site bytes" {
			t.Fatalf("prefixed staged bytes: %s %v", raw, err)
		}
		if h.uploadFail {
			return fmt.Errorf("Wrangler upload sentinel")
		}
		if h.badJSON {
			_, err := io.WriteString(stdout, "invalid provider output")
			return err
		}
		urls := []string{"https://branch.example.com", "https://feature-diagram-example-worker.team.workers.dev"}
		if h.noURLs {
			urls = nil
		}
		if h.unexpectedHost {
			urls = []string{"https://outside.invalid"}
		}
		return json.NewEncoder(stdout).Encode(map[string]any{"preview": previewResource{ID: "preview-id", URLs: urls}, "deployment": previewResource{ID: "failed-deployment", URLs: []string{"https://deployment.example.com"}}})
	}
}

func TestPreviewPrivateUploadAndDryRun(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(fmt.Sprint(dry), func(t *testing.T) {
			opts, m, workspace, h := previewFixture(t)
			opts.DryRun = dry
			var stdout bytes.Buffer
			opts.Stdout = &stdout
			before, err := os.ReadFile(filepath.Join(opts.Dir, opts.From, "build-manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := uploadArtifact(opts, h.run(t, m), h.client); err != nil {
				t.Fatal(err)
			}
			if dry && stdout.Len() != 0 || !dry && !strings.Contains(stdout.String(), "Private preview ready: https://deployment.example.com/blog%20space/\n") {
				t.Fatalf("readiness output: %s", stdout.String())
			}
			_, err = os.Stat(workspace)
			if dry && err != nil || !dry && !os.IsNotExist(err) {
				t.Fatalf("workspace lifecycle: %v", err)
			}
			after, err := os.ReadFile(filepath.Join(opts.Dir, opts.From, "build-manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("manifest changed")
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if dry {
				for _, r := range h.requests {
					if !strings.HasPrefix(r, "GET ") {
						t.Fatalf("dry run changed provider state: %s", r)
					}
				}
			}
			if !dry {
				for _, host := range []string{"branch.example.com", "deployment.example.com", "feature-diagram-example-worker.team.workers.dev"} {
					if !slices.Contains(h.requests, "GET "+host+"/blog%20space/en/") {
						t.Errorf("host was not checked: %s", host)
					}
				}
			}
			if h.uploads != 1 {
				t.Fatalf("upload calls: %d", h.uploads)
			}
		})
	}
}

func TestPreviewAccessFailuresBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *previewHarness, DeployOptions)
	}{
		{"wrong Worker", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.apps[0].Destinations[0].WorkerID = "other-worker"
		}},
		{"hostname only", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.apps[0].Destinations = []accessDestination{{Type: "public", URI: "*.example.com"}}
		}},
		{"bypass", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.policies = append(h.policies, accessPolicy{Decision: "bypass"})
		}},
		{"unknown policy", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.policies = []accessPolicy{{Decision: "unknown"}}
		}},
		{"empty policies", func(t *testing.T, h *previewHarness, o DeployOptions) { h.policies = []accessPolicy{} }},
		{"public path override", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.apps[0].Destinations[0].Overrides = []json.RawMessage{json.RawMessage(`{"behavior":"public","path_pattern":"/pagefind/*"}`)}
		}},
		{"overriding hostname bypass", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.apps = append(h.apps, accessApp{ID: "bypass", Type: "self_hosted", Destinations: []accessDestination{{Type: "public", URI: "future-branch.example.com/pagefind/*", Overrides: []json.RawMessage{json.RawMessage(`{"behavior":"public"}`)}}}})
		}},
		{"unprotected Service Auth", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.policies = []accessPolicy{{Decision: "non_identity", Include: []accessRule{{}}}}
		}},
		{"missing inspection token", func(t *testing.T, h *previewHarness, o DeployOptions) { t.Setenv("CFGB_CF_ACCESS_API_TOKEN", "") }},
		{"missing probe token", func(t *testing.T, h *previewHarness, o DeployOptions) { t.Setenv("CFGB_CF_ACCESS_CLIENT_SECRET", "") }},
		{"API failure", func(t *testing.T, h *previewHarness, o DeployOptions) {
			h.failPath = "/accounts/test-account/access/apps"
		}},
		{"Access disabled", func(t *testing.T, h *previewHarness, o DeployOptions) {
			if err := os.WriteFile(filepath.Join(o.Dir, "cfgb.yaml"), []byte(testConfig+"\nsecurity:\n  previewAccess: false\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, m, workspace, h := previewFixture(t)
			tc.mutate(t, h, opts)
			err := uploadArtifact(opts, h.run(t, m), h.client)
			if err == nil || !strings.Contains(err.Error(), "E_PREVIEW_ACCESS") || strings.Contains(err.Error(), "sensitive-provider-error-body") {
				t.Fatalf("gate error: %v", err)
			}
			if _, err := os.Stat(workspace); err != nil {
				t.Fatalf("preflight consumed workspace: %v", err)
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.uploads != 0 {
				t.Fatal("upload before Access verification")
			}
			for _, r := range h.requests {
				if !strings.HasPrefix(r, "GET ") {
					t.Fatalf("mutation before gate: %s", r)
				}
			}
		})
	}
}

func TestPreviewFailureRemovesOnlyAttemptDeployment(t *testing.T) {
	for _, kind := range []string{"anonymous HTML", "anonymous media", "anonymous feed", "anonymous Pagefind", "authorized wrong bytes", "no active URLs", "invalid Wrangler output", "upload error", "cleanup error", "unexpected host", "authorized redirect"} {
		t.Run(kind, func(t *testing.T) {
			opts, m, workspace, h := previewFixture(t)
			var stdout bytes.Buffer
			opts.Stdout = &stdout
			switch kind {
			case "anonymous HTML":
				h.leakPath = "/blog space/en/"
			case "anonymous media":
				h.leakPath = "/blog space/image.svg"
			case "anonymous feed":
				h.leakPath = "/blog space/en/feed.xml"
			case "anonymous Pagefind":
				h.leakPath = "/blog space/pagefind/pagefind.js"
			case "authorized wrong bytes":
				h.wrongPath = "/blog space/pagefind/pagefind.js"
			case "unexpected host":
				h.unexpectedHost = true
			case "authorized redirect":
				h.authorizedRedirect = true
			case "no active URLs":
				h.noURLs = true
			case "invalid Wrangler output":
				h.badJSON = true
			case "upload error":
				h.uploadFail = true
			case "cleanup error":
				h.uploadFail = true
				h.cleanupFail = true
			}
			err := uploadArtifact(opts, h.run(t, m), h.client)
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != 3 {
				t.Fatalf("post-upload error: %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed preview printed readiness: %s", stdout.String())
			}
			if kind == "cleanup error" && (!strings.Contains(err.Error(), "Wrangler upload sentinel") || !strings.Contains(err.Error(), "remove failed preview")) {
				t.Fatalf("lost primary/cleanup failure: %v", err)
			}
			if _, err := os.Stat(workspace); !os.IsNotExist(err) {
				t.Fatalf("upload did not consume session: %v", err)
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			deleted := false
			for _, r := range h.requests {
				if strings.HasPrefix(r, "DELETE ") {
					if !strings.HasSuffix(r, "/deployments/failed-deployment") {
						t.Fatalf("deleted unrelated preview/history: %s", r)
					}
					deleted = true
				}
			}
			if !deleted {
				t.Fatal("failed deployment cleanup not attempted")
			}
		})
	}
}

func TestPreviewBranchGateAndSummary(t *testing.T) {
	for _, kind := range []string{"production", "missing branch", "missing summary"} {
		t.Run(kind, func(t *testing.T) {
			opts, m, _, h := previewFixture(t)
			if kind == "missing summary" {
				m.Publications[0].Summary = ""
				writeDeployManifest(t, filepath.Join(opts.Dir, opts.From), m)
			} else {

				if kind == "production" {
					if _, err := output("git", "-C", opts.Dir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
						t.Fatal(err)
					}
					t.Setenv("WORKERS_CI_BRANCH", "feature/ignored")
				} else {
					if _, err := output("git", "-C", opts.Dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "test"); err != nil {
						t.Fatal(err)
					}
					if _, err := output("git", "-C", opts.Dir, "checkout", "--detach"); err != nil {
						t.Fatal(err)
					}
				}
			}
			err := uploadArtifact(opts, h.run(t, m), h.client)
			if err == nil {
				t.Fatal("expected gate failure")
			}
			if h.uploads != 0 {
				t.Fatal("uploaded rejected branch/content")
			}
		})
	}
}

func TestPreviewHostOverlap(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"*.example.com", "*.example.com", true}, {"specific.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false}, {"*other-worker.team.workers.dev", "*-example-worker.team.workers.dev", false},
		{"*.workers.dev", "*-example-worker.team.workers.dev", true}, {"*.unrelated.invalid", "*.example.com", false},
	} {
		if got := globOverlap(tc.a, tc.b); got != tc.want {
			t.Errorf("overlap(%q,%q)=%t", tc.a, tc.b, got)
		}
	}
}

func TestPreviewServiceAuthAndLoginRedirect(t *testing.T) {
	for _, kind := range []string{"token group", "require token", "anonymous alternative", "group cycle", "group read denied"} {
		t.Run(kind, func(t *testing.T) {
			opts, m, _, h := previewFixture(t)
			h.loginRedirect = true
			token := accessRule{AnyValidServiceToken: &struct{}{}}
			group := accessRule{Group: &struct {
				ID string `json:"id"`
			}{ID: "reviewed-group"}}
			h.policies = []accessPolicy{{Decision: "non_identity", Include: []accessRule{group}}}
			h.group = accessPolicy{Include: []accessRule{token}}
			wantError := false
			switch kind {
			case "require token":
				h.policies[0] = accessPolicy{Decision: "non_identity", Include: []accessRule{{}}, Require: []accessRule{token}}
			case "anonymous alternative":
				h.group.Include = append(h.group.Include, accessRule{})
				wantError = true
			case "group cycle":
				h.group.Include = []accessRule{group}
				wantError = true
			case "group read denied":
				h.failPath = "/accounts/test-account/access/groups/reviewed-group"
				wantError = true
			}
			err := uploadArtifact(opts, h.run(t, m), h.client)
			if (err != nil) != wantError {
				t.Fatalf("Service Auth verification: %v", err)
			}
		})
	}
}

func TestPreviewDetachedAndCustomProductionBranch(t *testing.T) {
	for _, kind := range []string{"detached", "custom production"} {
		t.Run(kind, func(t *testing.T) {
			opts, m, _, h := previewFixture(t)
			opts.DryRun = true
			if kind == "detached" {
				if _, err := output("git", "-C", opts.Dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "test"); err != nil {
					t.Fatal(err)
				}
				if _, err := output("git", "-C", opts.Dir, "checkout", "--detach"); err != nil {
					t.Fatal(err)
				}
				t.Setenv("WORKERS_CI_BRANCH", "feature/diagram")
			} else {
				if err := os.WriteFile(filepath.Join(opts.Dir, "cfgb.yaml"), []byte(testConfig+"\ndeploy:\n  productionBranch: release\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := output("git", "-C", opts.Dir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
					t.Fatal(err)
				}
				t.Setenv("WORKERS_CI_BRANCH", "release") // Live Git branch takes precedence.
			}
			if err := uploadArtifact(opts, h.run(t, m), h.client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPreviewOutputFailureRetainsVerifiedDeployment(t *testing.T) {
	opts, m, _, h := previewFixture(t)
	failure := errors.New("preview output unavailable")
	opts.Stdout = progressWriter(func([]byte) (int, error) { return 0, failure })
	err := uploadArtifact(opts, h.run(t, m), h.client)
	if !errors.Is(err, failure) {
		t.Fatalf("lost output error: %v", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, request := range h.requests {
		if strings.HasPrefix(request, "DELETE ") {
			t.Fatalf("output error deleted a verified deployment: %s", request)
		}
	}
}

func TestCloudflareTotalCountPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		items := []string{"first", "second"}
		if r.URL.Query().Get("page") == "2" {
			page = 2
			items = []string{"last"}
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "result": items, "result_info": map[string]int{"page": page, "total_count": 3}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := &previewClient{ctx: context.Background(), apiBase: server.URL, http: server.Client()}
	items, err := cfList[string](c, "/collection", "token")
	if err != nil || !slices.Equal(items, []string{"first", "second", "last"}) {
		t.Fatalf("paginated results: %v %v", items, err)
	}
}

func TestPreviewStaticArtifactRedirects(t *testing.T) {
	for _, kind := range []string{"expected redirects", "wrong alias target", "external root", "anonymous root leak", "anonymous alias leak"} {
		t.Run(kind, func(t *testing.T) {
			opts, m, _, h := previewFixture(t)
			h.redirects = map[string]struct {
				status   int
				location string
			}{
				"/blog space/":        {302, "/blog%20space/en/"},
				"/blog space":         {302, "/blog%20space/en/"},
				"/blog space/en/old/": {301, "/blog%20space/en/"},
			}
			for name, body := range map[string]string{"index.html": "static root redirect document", "en/old/index.html": "static alias redirect document", "_redirects": "/blog%20space/en/old/ /blog%20space/en/ 301\n"} {
				path := filepath.Join(opts.Dir, opts.From, "site", filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			wantError := kind != "expected redirects"
			switch kind {
			case "wrong alias target":
				h.redirects["/blog space/en/old/"] = struct {
					status   int
					location string
				}{301, "/blog%20space/en/missing/"}
			case "external root":
				h.redirects["/blog space/"] = struct {
					status   int
					location string
				}{302, "https://public.invalid/blog%20space/en/"}
			case "anonymous root leak":
				h.leakPath = "/blog space/"
			case "anonymous alias leak":
				h.leakPath = "/blog space/en/old/"
			}
			err := uploadArtifact(opts, h.run(t, m), h.client)
			if (err != nil) != wantError {
				t.Fatalf("static artifact readiness: %v", err)
			}
		})
	}
}

func TestPreviewReadinessPolicyBeforeMutation(t *testing.T) {
	for _, dry := range []bool{false, true} {
		for _, kind := range []string{"human only", "deny only", "certificate only", "other token", "excluded token"} {
			t.Run(fmt.Sprintf("%s/dry=%t", kind, dry), func(t *testing.T) {
				opts, m, workspace, h := previewFixture(t)
				opts.DryRun = dry
				switch kind {
				case "human only":
					h.policies = []accessPolicy{{Decision: "allow"}}
				case "deny only":
					h.policies = []accessPolicy{{Decision: "deny"}}
				case "certificate only":
					h.policies = []accessPolicy{{Decision: "non_identity", Include: []accessRule{{Certificate: &struct{}{}}}}}
				case "other token":
					h.policies = []accessPolicy{{Decision: "non_identity", Include: []accessRule{{ServiceToken: &struct {
						TokenID string `json:"token_id"`
					}{TokenID: "different-token"}}}}}
					h.serviceTokens = []map[string]any{{"id": "readiness-token", "client_id": "probe-client"}}
				case "excluded token":
					var policy accessPolicy
					if err := json.Unmarshal([]byte(`{"decision":"non_identity","include":[{"any_valid_service_token":{}}],"exclude":[{"any_valid_service_token":{}}]}`), &policy); err != nil {
						t.Fatal(err)
					}
					h.policies = []accessPolicy{policy}

				}
				err := uploadArtifact(opts, h.run(t, m), h.client)
				if err == nil || !strings.Contains(err.Error(), "E_PREVIEW_ACCESS") {
					t.Fatalf("readiness preflight: %v", err)
				}
				if _, err := os.Stat(workspace); err != nil {
					t.Fatalf("preflight consumed session: %v", err)
				}
				h.mu.Lock()
				defer h.mu.Unlock()
				if h.uploads != 0 {
					t.Fatal("upload before readiness-policy check")
				}
				for _, r := range h.requests {
					if !strings.HasPrefix(r, "GET ") {
						t.Fatalf("mutated before policy check: %s", r)
					}
				}
			})
		}
	}
}

func TestPreviewReadinessTokenPolicies(t *testing.T) {
	for _, tc := range []struct {
		name, policy string
		wantError    bool
	}{
		{"matching token", `{"include":[{"service_token":{"token_id":"readiness-token"}}]}`, false},
		{"different exclusion", `{"include":[{"any_valid_service_token":{}}],"exclude":[{"service_token":{"token_id":"another-token"}}]}`, false},
		{"matching exclusion", `{"include":[{"any_valid_service_token":{}}],"exclude":[{"service_token":{"token_id":"readiness-token"}}]}`, true},
		{"human requirement", `{"include":[{"service_token":{"token_id":"readiness-token"}}],"require":[{"certificate":{}}]}`, true},
		{"network condition", `{"include":[{"any_valid_service_token":{}}],"require":[{"ip":{"ip":"203.0.113.0/24"}}]}`, false},
		{"group exclusion", `{"include":[{"any_valid_service_token":{}}],"exclude":[{"group":{"id":"excluded-group"}}]}`, true},
		{"alternative token path", `{"include":[{"service_token":{"token_id":"another-token"}}]}`, false},
		{"inspection denied", `{"include":[{"service_token":{"token_id":"readiness-token"}}]}`, true},
		{"expired token", `{"include":[{"service_token":{"token_id":"readiness-token"}}]}`, true},
		{"disabled token", `{"include":[{"service_token":{"token_id":"readiness-token"}}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, m, workspace, h := previewFixture(t)
			var policy accessPolicy
			if err := json.Unmarshal([]byte(tc.policy), &policy); err != nil {
				t.Fatal(err)
			}
			policy.Decision = "non_identity"
			h.policies = []accessPolicy{policy}
			h.serviceTokens = []map[string]any{{"id": "readiness-token", "client_id": "probe-client"}}
			h.group = accessPolicy{Include: []accessRule{{AnyValidServiceToken: &struct{}{}}}}
			switch tc.name {
			case "alternative token path":
				h.failPath = "/accounts/test-account/access/service_tokens"
				h.policies = append(h.policies, accessPolicy{Decision: "non_identity", Include: []accessRule{{AnyValidServiceToken: &struct{}{}}}})
			case "inspection denied":
				h.failPath = "/accounts/test-account/access/service_tokens"
			case "expired token":
				h.serviceTokens[0]["expires_at"] = "2000-01-01T00:00:00Z"
			case "disabled token":
				h.serviceTokens[0]["enabled"] = false
			}
			err := uploadArtifact(opts, h.run(t, m), h.client)
			if (err != nil) != tc.wantError {
				t.Fatalf("token policy: %v", err)
			}
			if tc.wantError {
				if _, err := os.Stat(workspace); err != nil {
					t.Fatalf("policy failure consumed session: %v", err)
				}
				h.mu.Lock()
				defer h.mu.Unlock()
				for _, r := range h.requests {
					if !strings.HasPrefix(r, "GET ") {
						t.Fatalf("mutation before token policy check: %s", r)
					}
				}
			}
		})
	}
}

func TestPreviewUnrelatedPrivateHostnamePolicy(t *testing.T) {
	opts, m, _, h := previewFixture(t)
	h.apps = append(h.apps, accessApp{ID: "human", Type: "self_hosted", Domain: "future.example.com/blog/search"})
	h.policiesByApp = map[string][]accessPolicy{"/accounts/test-account/access/apps/human/policies": {{Decision: "allow"}}}
	if err := uploadArtifact(opts, h.run(t, m), h.client); err != nil {
		t.Fatalf("unrelated private hostname blocked current preview: %v", err)
	}
}

func TestPreviewReadinessIncludeAlternatives(t *testing.T) {
	for _, dry := range []bool{false, true} {
		for _, explicitFirst := range []bool{false, true} {
			for _, condition := range []string{"admitted", "required certificate", "excluded token", "required lookup", "excluded lookup", "no matching alternative"} {
				t.Run(fmt.Sprintf("dry=%t/explicit-first=%t/%s", dry, explicitFirst, condition), func(t *testing.T) {
					opts, manifest, workspace, h := previewFixture(t)
					opts.DryRun = dry
					h.failPath = "/accounts/test-account/access/service_tokens"
					var explicit accessRule
					if err := json.Unmarshal([]byte(`{"service_token":{"token_id":"named-token"}}`), &explicit); err != nil {
						t.Fatal(err)
					}
					anyToken := accessRule{AnyValidServiceToken: &struct{}{}}
					include := []accessRule{anyToken, explicit}
					if explicitFirst {
						slices.Reverse(include)
					}
					policy := accessPolicy{Decision: "non_identity", Include: include}
					switch condition {
					case "required certificate":
						policy.Require = []accessRule{{Certificate: &struct{}{}}}
					case "excluded token":
						policy.Exclude = []accessRule{anyToken}
					case "required lookup":
						policy.Require = []accessRule{explicit}
					case "excluded lookup":
						policy.Exclude = []accessRule{explicit}
					case "no matching alternative":
						policy.Include = []accessRule{explicit}
					}
					h.policies = []accessPolicy{policy}
					err := uploadArtifact(opts, h.run(t, manifest), h.client)
					if condition == "admitted" {
						if err != nil {
							t.Fatalf("Include alternative required unnecessary permission: %v", err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), "E_PREVIEW_ACCESS") {
						t.Fatalf("Require/Exclude/lookup gate: %v", err)
					}
					if strings.Contains(condition, "lookup") || condition == "no matching alternative" {
						var apiErr *cfAPIError
						if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
							t.Fatalf("lost required lookup error: %v", err)
						}
					}
					if _, err := os.Stat(workspace); err != nil {
						t.Fatalf("preflight consumed toolchain: %v", err)
					}
					h.mu.Lock()
					defer h.mu.Unlock()
					if h.uploads != 0 {
						t.Fatal("uploaded despite Require/Exclude failure")
					}
					for _, request := range h.requests {
						if !strings.HasPrefix(request, "GET ") {
							t.Fatalf("mutation before gate: %s", request)
						}
					}
				})
			}
		}
	}
}
