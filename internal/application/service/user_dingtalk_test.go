package service

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

func TestDingTalkSubjectPrefersUnionID(t *testing.T) {
	info := &dingTalkUserInfo{UnionID: "union-1", OpenID: "open-1"}
	if got := dingTalkSubject(info); got != "union-1" {
		t.Fatalf("dingTalkSubject() = %q, want union-1", got)
	}

	info.UnionID = ""
	if got := dingTalkSubject(info); got != "open-1" {
		t.Fatalf("dingTalkSubject() fallback = %q, want open-1", got)
	}
}

func TestDingTalkFallbackEmailIsStableAndReserved(t *testing.T) {
	first := dingTalkFallbackEmail("union-stable")
	second := dingTalkFallbackEmail("union-stable")
	other := dingTalkFallbackEmail("union-other")

	if first != second {
		t.Fatalf("fallback email is not stable: %q != %q", first, second)
	}
	if first == other {
		t.Fatalf("different subjects produced the same fallback email: %q", first)
	}
	if !strings.HasSuffix(first, "@external.invalid") {
		t.Fatalf("fallback email = %q, want reserved .invalid domain", first)
	}
}

func TestGetDingTalkAuthorizationURL(t *testing.T) {
	svc := &userService{
		config: &config.Config{
			DingTalkAuth: &config.DingTalkAuthConfig{
				Enable:              true,
				ProviderDisplayName: "钉钉",
				ClientID:            "ding-client",
				ClientSecret:        "secret",
				CorpID:              "ding-corp",
			},
		},
	}

	response, err := svc.GetDingTalkAuthorizationURL(
		context.Background(),
		"https://weknora.example.com/api/v1/auth/dingtalk/callback",
	)
	if err != nil {
		t.Fatalf("GetDingTalkAuthorizationURL() error = %v", err)
	}
	if !response.Success || response.Nonce == "" || response.State == "" {
		t.Fatalf("incomplete response: %#v", response)
	}

	parsed, err := url.Parse(response.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "login.dingtalk.com" || parsed.Path != "/oauth2/auth" {
		t.Fatalf("unexpected authorization endpoint: %s", parsed.String())
	}
	query := parsed.Query()
	if query.Get("client_id") != "ding-client" {
		t.Fatalf("client_id = %q", query.Get("client_id"))
	}
	if query.Get("response_type") != "code" {
		t.Fatalf("response_type = %q", query.Get("response_type"))
	}
	if query.Get("scope") != "openid corpid" {
		t.Fatalf("scope = %q", query.Get("scope"))
	}
	if query.Get("state") != response.State {
		t.Fatalf("state mismatch")
	}
}
