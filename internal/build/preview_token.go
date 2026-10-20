package build

import (
	"fmt"
	"net/url"
	"time"
)

// Policy metadata can prove a token cannot match (wrong token, a certificate
// requirement, or an exclusion). Network/device conditions remain unknown until
// the post-upload request, as does the validity of the supplied client secret.
type tokenMatch int

const (
	tokenNo tokenMatch = iota
	tokenYes
	tokenUnknown
)

func tokenAnd(a, b tokenMatch) tokenMatch {
	if a == tokenNo || b == tokenNo {
		return tokenNo
	}
	if a == tokenYes && b == tokenYes {
		return tokenYes
	}
	return tokenUnknown
}

func tokenOr(a, b tokenMatch) tokenMatch {
	if a == tokenYes || b == tokenYes {
		return tokenYes
	}
	if a == tokenNo && b == tokenNo {
		return tokenNo
	}
	return tokenUnknown
}

type readinessMatcher struct {
	client                       *previewClient
	account, inspector, clientID string
	tokenID                      string
}

func (m *readinessMatcher) policy(policy accessPolicy, active map[string]bool) (tokenMatch, error) {
	include := tokenNo
	for _, rule := range policy.Include {
		match, err := m.rule(rule, active)
		if err != nil {
			return tokenUnknown, err
		}
		include = tokenOr(include, match)
	}
	for _, rule := range policy.Require {
		match, err := m.rule(rule, active)
		if err != nil {
			return tokenUnknown, err
		}
		include = tokenAnd(include, match)
	}
	for _, rule := range policy.Exclude {
		match, err := m.rule(rule, active)
		if err != nil {
			return tokenUnknown, err
		}
		if match == tokenYes {
			return tokenNo, nil
		}
		if match == tokenUnknown {
			include = tokenAnd(include, tokenUnknown)
		}
	}
	return include, nil
}

func (m *readinessMatcher) rule(rule accessRule, active map[string]bool) (tokenMatch, error) {
	if rule.Everyone != nil || rule.AnyValidServiceToken != nil {
		return tokenYes, nil
	}
	if rule.Certificate != nil || rule.CommonName != nil {
		return tokenNo, nil
	}
	if rule.ServiceToken != nil {
		id, err := m.identity()
		if err != nil {
			return tokenUnknown, err
		}
		if rule.ServiceToken.TokenID == id {
			return tokenYes, nil
		}
		return tokenNo, nil
	}
	if rule.Group != nil && rule.Group.ID != "" {
		if active[rule.Group.ID] {
			return tokenUnknown, fmt.Errorf("access group cycle prevents readiness verification")
		}
		active[rule.Group.ID] = true
		defer delete(active, rule.Group.ID)
		var group accessPolicy
		if _, err := m.client.api("GET", m.account+"/access/groups/"+url.PathEscape(rule.Group.ID), m.inspector, nil, &group); err != nil {
			return tokenUnknown, err
		}
		return m.policy(group, active)
	}
	return tokenUnknown, nil
}

func (m *readinessMatcher) identity() (string, error) {
	if m.tokenID != "" {
		return m.tokenID, nil
	}
	tokens, err := cfList[struct {
		ID        string `json:"id"`
		ClientID  string `json:"client_id"`
		Enabled   *bool  `json:"enabled"`
		ExpiresAt string `json:"expires_at"`
	}](m.client, m.account+"/access/service_tokens", m.inspector)
	if err != nil {
		return "", fmt.Errorf("resolve readiness client ID (Access: Service Tokens Read is needed for explicit token policies): %w", err)
	}
	for _, token := range tokens {
		if token.ClientID != m.clientID {
			continue
		}
		if token.ID == "" || (token.Enabled != nil && !*token.Enabled) {
			return "", fmt.Errorf("readiness service token is missing its identity or is disabled")
		}
		if token.ExpiresAt != "" {
			expires, err := time.Parse(time.RFC3339, token.ExpiresAt)
			if err != nil {
				return "", fmt.Errorf("read readiness service-token expiry: %w", err)
			}
			if !expires.After(time.Now()) {
				return "", fmt.Errorf("readiness service token has expired")
			}
		}
		m.tokenID = token.ID
		return token.ID, nil
	}
	return "", fmt.Errorf("readiness client ID was not found among inspected service tokens")
}
