package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ymmt2005/cfgb/internal/config"
)

// PreviewOptions selects an existing artifact for a private branch preview.
// DryRun performs Access inspection and local Wrangler validation only.
type PreviewOptions = DeployOptions

// Preview uploads only after verifying previews-only Cloudflare Access coverage.
func Preview(opts PreviewOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := &previewClient{ctx: ctx, apiBase: "https://api.cloudflare.com/client/v4", http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	return uploadArtifact(opts, command, client)
}

type previewClient struct {
	ctx     context.Context
	apiBase string
	http    *http.Client
}

type previewTarget struct {
	workerPath, branch, uploadToken, accessID, accessSecret string
	hosts                                                   []string
}

type accessDestination struct {
	Type      string            `json:"type"`
	WorkerID  string            `json:"worker_id"`
	URI       string            `json:"uri"`
	Overrides []json.RawMessage `json:"overrides"`
}

type accessApp struct {
	ID                string              `json:"id"`
	Type              string              `json:"type"`
	Domain            string              `json:"domain"`
	Destinations      []accessDestination `json:"destinations"`
	SelfHostedDomains []string            `json:"self_hosted_domains"`
}

type accessPolicy struct {
	Decision string       `json:"decision"`
	Include  []accessRule `json:"include"`
	Require  []accessRule `json:"require"`
}

type accessRule struct {
	ServiceToken *struct {
		TokenID string `json:"token_id"`
	} `json:"service_token"`
	AnyValidServiceToken *struct{} `json:"any_valid_service_token"`
	Certificate          *struct{} `json:"certificate"`
	CommonName           *struct {
		CommonName string `json:"common_name"`
	} `json:"common_name"`
	Group *struct {
		ID string `json:"id"`
	} `json:"group"`
}

// Allow requires user authentication. Service Auth can instead authorize by
// network location alone, so prove that every accepting path requires a token
// or certificate. Unknown selectors cannot establish that proof. Excludes only
// narrow access and do not turn an unauthenticated path into authentication.
func (c *previewClient) policyAuthenticated(account, token string, policy accessPolicy, active map[string]bool) (bool, error) {
	for _, rule := range policy.Require {
		authenticated, err := c.ruleAuthenticated(account, token, rule, active)
		if err != nil {
			return false, err
		}
		if authenticated {
			return true, nil
		}
	}
	if len(policy.Include) == 0 {
		return false, nil
	}
	for _, rule := range policy.Include {
		authenticated, err := c.ruleAuthenticated(account, token, rule, active)
		if err != nil || !authenticated {
			return false, err
		}
	}
	return true, nil
}

func (c *previewClient) ruleAuthenticated(account, token string, rule accessRule, active map[string]bool) (bool, error) {
	if (rule.ServiceToken != nil && rule.ServiceToken.TokenID != "") || rule.AnyValidServiceToken != nil || rule.Certificate != nil || (rule.CommonName != nil && rule.CommonName.CommonName != "") {
		return true, nil
	}
	if rule.Group == nil || rule.Group.ID == "" {
		return false, nil
	}
	if active[rule.Group.ID] {
		return false, fmt.Errorf("access group cycle prevents verification")
	}
	active[rule.Group.ID] = true
	defer delete(active, rule.Group.ID)
	var group accessPolicy
	if _, err := c.api("GET", account+"/access/groups/"+url.PathEscape(rule.Group.ID), token, nil, &group); err != nil {
		return false, err
	}
	return c.policyAuthenticated(account, token, group, active)
}

type cfPage struct{ Page, TotalPages, TotalCount int }
type cfAPIError struct{ Status int }

func (e *cfAPIError) Error() string { return fmt.Sprintf("Cloudflare API returned HTTP %d", e.Status) }

// Do not echo response bodies: provider errors can contain sensitive inputs.
func (c *previewClient) api(method, path, token string, body, result any) (page cfPage, err error) {
	var raw []byte
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return page, fmt.Errorf("encode Cloudflare request: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.apiBase+path, bytes.NewReader(raw))
	if err != nil {
		return page, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return page, fmt.Errorf("cloudflare %s %s: %w", method, path, err)
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return page, &cfAPIError{Status: resp.StatusCode}
	}
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Info    struct {
			Page       int `json:"page"`
			TotalPages int `json:"total_pages"`
			TotalCount int `json:"total_count"`
		} `json:"result_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return page, fmt.Errorf("decode Cloudflare response: %w", err)
	}
	if !envelope.Success {
		return page, fmt.Errorf("cloudflare %s %s did not succeed", method, path)
	}
	if result != nil {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return page, fmt.Errorf("decode Cloudflare result: %w", err)
		}
	}
	return cfPage{envelope.Info.Page, envelope.Info.TotalPages, envelope.Info.TotalCount}, nil
}

func cfList[T any](c *previewClient, path, token string) ([]T, error) {
	var all []T
	for number := 1; ; number++ {
		var items []T
		page, err := c.api("GET", fmt.Sprintf("%s?page=%d&per_page=100", path, number), token, nil, &items)
		if err != nil {
			return nil, err
		}
		if items == nil {
			return nil, fmt.Errorf("cloudflare list %s omitted its result", path)
		}
		all = append(all, items...)
		if page.TotalPages > 0 && number >= page.TotalPages {
			return all, nil
		}
		if page.TotalPages == 0 {
			if page.TotalCount > 0 && len(all) >= page.TotalCount {
				return all, nil
			}
			if page.TotalCount == 0 && len(items) < 100 {
				return all, nil
			}
			if page.TotalCount == 0 {
				return nil, fmt.Errorf("cloudflare list %s omitted pagination for a full page", path)
			}
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("cloudflare list %s ended before its advertised total", path)
		}
		if page.Page != number {
			return nil, fmt.Errorf("cloudflare list %s returned inconsistent pagination", path)
		}
	}
}

func previewAccess(err error) error {
	return &ExitError{Code: 1, Err: fmt.Errorf("E_PREVIEW_ACCESS: %w", err)}
}

func (c *previewClient) preflight(cfg *config.File, account, name, branch string) (*previewTarget, error) {
	if !cfg.Security.PreviewAccess {
		return nil, &ExitError{Code: 2, Err: fmt.Errorf("E_PREVIEW_ACCESS: security.previewAccess must be true")}
	}
	inspect := os.Getenv("CFGB_CF_ACCESS_API_TOKEN")
	t := &previewTarget{uploadToken: os.Getenv("CLOUDFLARE_API_TOKEN"), accessID: os.Getenv("CFGB_CF_ACCESS_CLIENT_ID"), accessSecret: os.Getenv("CFGB_CF_ACCESS_CLIENT_SECRET")}
	if inspect == "" || t.accessID == "" || t.accessSecret == "" || t.uploadToken == "" {
		return nil, previewAccess(fmt.Errorf("set CFGB_CF_ACCESS_API_TOKEN, CFGB_CF_ACCESS_CLIENT_ID, CFGB_CF_ACCESS_CLIENT_SECRET and CLOUDFLARE_API_TOKEN (also required for preview --dry-run inspection)"))
	}
	t.branch = branch
	accountPath := "/accounts/" + url.PathEscape(account)
	var worker struct {
		ID, Name  string
		Subdomain struct {
			PreviewURLSuffix string `json:"preview_url_suffix"`
		} `json:"subdomain"`
		References struct {
			Domains []struct {
				Hostname string `json:"hostname"`
			} `json:"domains"`
		} `json:"references"`
	}
	if _, err := c.api("GET", accountPath+"/workers/workers/"+url.PathEscape(name), t.uploadToken, nil, &worker); err != nil {
		return nil, previewAccess(fmt.Errorf("resolve existing Worker: %w", err))
	}
	if worker.ID == "" || worker.Name != name || worker.References.Domains == nil {
		return nil, previewAccess(fmt.Errorf("could not resolve the target Worker identity and custom-domain references"))
	}
	t.workerPath = accountPath + "/workers/workers/" + url.PathEscape(worker.ID)
	if worker.Subdomain.PreviewURLSuffix != "" {
		t.hosts = append(t.hosts, "*"+worker.Subdomain.PreviewURLSuffix)
	}
	for _, domain := range worker.References.Domains {
		t.hosts = append(t.hosts, "*."+domain.Hostname)
	}
	if len(t.hosts) == 0 {
		return nil, previewAccess(fmt.Errorf("worker has no prospective Preview URL host; provision a workers.dev subdomain or custom preview domain"))
	}
	apps, err := cfList[accessApp](c, accountPath+"/access/apps", inspect)
	if err != nil {
		return nil, previewAccess(fmt.Errorf("inspect Access applications: %w", err))
	}
	covered := false
	for _, app := range apps {
		relevant := false
		for _, destination := range app.Destinations {
			if destination.Type == "preview_worker" && destination.WorkerID == worker.ID {
				covered, relevant = true, true
				if len(destination.Overrides) != 0 {
					return nil, previewAccess(fmt.Errorf("access application %s has public path overrides", app.ID))
				}
			}
			if (destination.Type == "public" || destination.Type == "") && overlapsHosts(destination.URI, t.hosts) {
				relevant = true
				if len(destination.Overrides) != 0 {
					return nil, previewAccess(fmt.Errorf("access application %s has public path overrides on preview hosts", app.ID))
				}
			}
		}
		if len(app.Destinations) == 0 {
			relevant = overlapsHosts(app.Domain, t.hosts)
			for _, domain := range app.SelfHostedDomains {
				relevant = relevant || overlapsHosts(domain, t.hosts)
			}
		}
		if !relevant {
			continue
		}
		if app.ID == "" || app.Type != "self_hosted" {
			return nil, previewAccess(fmt.Errorf("cannot verify preview Access application %s", app.ID))
		}
		policies, err := cfList[accessPolicy](c, accountPath+"/access/apps/"+url.PathEscape(app.ID)+"/policies", inspect)
		if err != nil {
			return nil, previewAccess(fmt.Errorf("inspect effective policies for %s: %w", app.ID, err))
		}
		if len(policies) == 0 {
			return nil, previewAccess(fmt.Errorf("access application %s has no verifiable policies", app.ID))
		}
		for _, policy := range policies {
			switch policy.Decision {
			case "allow", "deny":
			case "non_identity":
				authenticated, err := c.policyAuthenticated(accountPath, inspect, policy, map[string]bool{})
				if err != nil {
					return nil, previewAccess(fmt.Errorf("inspect Service Auth policy in %s: %w", app.ID, err))
				}
				if !authenticated {
					return nil, previewAccess(fmt.Errorf("service auth policy in %s does not require verifiable credentials", app.ID))
				}
			default:
				return nil, previewAccess(fmt.Errorf("access application %s has a bypass or unverifiable policy", app.ID))
			}
		}
	}
	if !covered {
		return nil, previewAccess(fmt.Errorf("no previews-only Access destination (preview_worker) covers Worker %s", worker.ID))
	}
	return t, nil
}

// Public hostname/path applications take precedence over Worker destinations.
// Compare their hostname languages with every prospective preview hostname,
// including names not created yet. A path-specific bypass is still a leak.
func overlapsHosts(uri string, hosts []string) bool {
	uri = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(uri), "https://"), "http://")
	host, _, _ := strings.Cut(uri, "/")
	if host == "" {
		return false
	}
	for _, candidate := range hosts {
		if globOverlap(host, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func globOverlap(a, b string) bool {
	seen := map[[2]int]bool{}
	var visit func(int, int) bool
	visit = func(i, j int) bool {
		key := [2]int{i, j}
		if seen[key] {
			return false
		}
		seen[key] = true
		if i == len(a) && j == len(b) {
			return true
		}
		if i < len(a) && a[i] == '*' && visit(i+1, j) {
			return true
		}
		if j < len(b) && b[j] == '*' && visit(i, j+1) {
			return true
		}
		if i == len(a) || j == len(b) {
			return false
		}
		if a[i] != '*' && b[j] != '*' && a[i] != b[j] {
			return false
		}
		nextI, nextJ := i+1, j+1
		if a[i] == '*' {
			nextI = i
		}
		if b[j] == '*' {
			nextJ = j
		}
		return visit(nextI, nextJ)
	}
	return visit(0, 0)
}

func (c *previewClient) enable(t *previewTarget) error {
	_, err := c.api("PATCH", t.workerPath, t.uploadToken, map[string]any{"subdomain": map[string]bool{"enabled": false, "previews_enabled": true}}, nil)
	if err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("enable verified Preview URLs: %w", err)}
	}
	return nil
}
