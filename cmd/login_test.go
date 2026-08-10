package cmd

import (
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

// The browser flows share an identity provider, so the first one authenticates
// and the rest complete silently. Running them last wastes that.
func TestOrderPutsBrowserFlowsFirst(t *testing.T) {
	tools := []manifest.Tool{
		{Name: "opus", Auth: &manifest.Auth{Kind: "token"}},
		{Name: "sablier", Auth: &manifest.Auth{Kind: "sso"}},
		{Name: "antenne", Auth: &manifest.Auth{Kind: "password"}},
		{Name: "casier", Auth: &manifest.Auth{Kind: "sso"}},
	}

	got := make([]string, 0, len(tools))
	for _, tool := range order(tools) {
		got = append(got, tool.Name)
	}

	want := []string{"sablier", "casier", "opus", "antenne"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}

// The catalog holds the path; the server URL is only known at login time.
func TestTokenPageIsBuiltFromTheResolvedServer(t *testing.T) {
	tool := manifest.Tool{Auth: &manifest.Auth{TokenPath: "/settings/api"}}

	if got := tool.TokenPage("https://nuage.example.com/"); got != "https://nuage.example.com/settings/api" {
		t.Fatalf("token page is %q", got)
	}
	if got := tool.TokenPage(""); got != "" {
		t.Fatalf("no server means no page, got %q", got)
	}
	bare := manifest.Tool{Auth: &manifest.Auth{}}
	if got := bare.TokenPage("https://x.example.com"); got != "" {
		t.Fatalf("no path means no page, got %q", got)
	}
}
