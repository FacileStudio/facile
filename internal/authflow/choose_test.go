package authflow

import (
	"path/filepath"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

// flowCase is one catalog entry, one discovery answer and one provider, which
// together are everything chooseFlow looks at. The sessions are seeded by hand
// so the check is which flow an answer selects, not whether discovery works.
type flowCase struct {
	name    string
	auth    *manifest.Auth
	found   discovery
	session *Session
	want    string
	wantErr bool
}

func flowCases() []flowCase {
	sso := &manifest.Auth{Kind: "sso", Flows: manifest.Flows{SSO: &manifest.SSOFlow{}}}
	both := &manifest.Auth{Kind: "sso", Flows: manifest.Flows{
		SSO:      &manifest.SSOFlow{},
		Password: &manifest.PasswordFlow{Path: "/auth/login", WithEmail: true},
	}}
	password := &manifest.Auth{Kind: "password", Flows: manifest.Flows{
		Password: &manifest.PasswordFlow{Path: "/api/login"},
	}}
	device := &manifest.Auth{Kind: "oidc-device", Flows: manifest.Flows{
		SSO:        &manifest.SSOFlow{},
		OIDCDevice: &manifest.OIDCDeviceFlow{Issuer: "https://provider.test", ClientID: "facile-cli"},
	}}
	noIssuer := &manifest.Auth{Kind: "oidc-device", Flows: manifest.Flows{
		SSO: &manifest.SSOFlow{}, OIDCDevice: &manifest.OIDCDeviceFlow{},
	}}

	offers, refuses := seededSessions()

	return []flowCase{
		{"an unreachable discovery keeps the declared kind", sso, discovery{}, nil, "sso", false},
		{"sso_only with no provider is a dead end", sso, discovery{answered: true, ssoOnly: true}, nil, "", true},
		{"no OIDC falls back to the password endpoint", both, discovery{answered: true}, nil, "password", false},
		{"an instance with no password needs no prompt", password, discovery{answered: true}, nil, "passwordless", false},
		{"an instance with a password asks for it", password,
			discovery{answered: true, passwordNeeded: true}, nil, "password", false},
		{"a provider offering RFC 8628 takes the device grant", device, discovery{}, offers, "oidc-device", false},
		{"a provider without the grant keeps the loopback flow", device, discovery{}, refuses, "sso", false},
		{"a device kind with no issuer is a catalog bug", noIssuer, discovery{}, refuses, "", true},
	}
}

// seededSessions are the two answers a provider can give, without a provider.
// An issuer already in the cache is never discovered, so these decide the flow
// without a network call.
func seededSessions() (offers, refuses *Session) {
	offers = &Session{issuers: map[string]endpoints{"https://provider.test": {
		deviceAuthorization: "https://provider.test/device_authorization",
		token:               "https://provider.test/oauth/token",
		offersDeviceGrant:   true,
	}}}
	refuses = &Session{issuers: map[string]endpoints{"https://provider.test": {}}}
	return offers, refuses
}

func TestChooseFlow(t *testing.T) {
	for _, c := range flowCases() {
		t.Run(c.name, func(t *testing.T) {
			got, err := chooseFlow(c.auth, c.found, c.session)
			switch {
			case c.wantErr && err == nil:
				t.Fatalf("expected a refusal, got %q", got)
			case c.wantErr:
			case err != nil:
				t.Fatal(err)
			case got != c.want:
				t.Fatalf("chooseFlow = %q, want %q", got, c.want)
			}
		})
	}
}

// TestCatalogFlowsAreComplete keeps the catalog and the implementation honest:
// every tool that declares a login must carry the pieces that flow needs.
func TestCatalogFlowsAreComplete(t *testing.T) {
	for _, tool := range manifest.Load(filepath.Join(t.TempDir(), "absent.yml")).Tools {
		if !tool.NeedsLogin() {
			continue
		}
		a := tool.Auth
		if a.Store == nil {
			t.Errorf("%s declares a login with nowhere to store the result", tool.Name)
		}
		switch a.Kind {
		case "oidc-device", "sso":
			checkFederated(t, tool)
		case "password":
			checkPassword(t, tool)
		case "device":
			if a.Device == nil || a.Device.StartPath == "" || a.Device.PollPath == "" {
				t.Errorf("%s declares a device login with missing endpoints", tool.Name)
			}
		case "token":
		default:
			t.Errorf("%s declares unknown login kind %q", tool.Name, a.Kind)
		}
		if a.Store != nil && a.Store.Kind == "keychain" && a.Store.KeychainService == "" {
			t.Errorf("%s stores in a keychain with no service name", tool.Name)
		}
	}
}

// checkFederated covers both kinds that reach the shared provider, because the
// device flow is a preference and never a replacement: the loopback flow is
// what runs until the provider advertises the grant, so a tool that declares
// the device kind still has to declare a working browser flow.
//
// The code flow is porte's login-code contract and its parameters are fixed,
// not per-server taste. Enforcing them stops a catalog entry drifting back to
// the stale loopback-token spelling (cli_port, /callback, callbackWith token),
// which passes a token-less check and then times out at login.
func checkFederated(t *testing.T, tool manifest.Tool) {
	t.Helper()
	a := tool.Auth
	if a.SSO == nil || a.SSO.StartPath == "" {
		t.Fatalf("%s drops to the browser flow and declares none", tool.Name)
	}
	if a.Kind == "oidc-device" {
		if a.OIDCDevice == nil || a.OIDCDevice.Issuer == "" || a.OIDCDevice.ClientID == "" {
			t.Fatalf("%s declares a device sign-in with no issuer or client", tool.Name)
		}
		if a.OIDCDevice.ExchangePath == "" {
			t.Errorf("%s has nowhere to trade the provider's token for its own", tool.Name)
		}
	}
	if a.SSO.CallbackWith != "code" {
		return
	}
	if a.SSO.ExchangePath == "" {
		t.Errorf("%s exchanges a code with no exchange path", tool.Name)
	}
	if a.SSO.PortParam != "port" || a.SSO.StateParam != "cli_state" {
		t.Errorf("%s spells the port %q and the nonce %q, want port and cli_state",
			tool.Name, a.SSO.PortParam, a.SSO.StateParam)
	}
	if a.SSO.ExtraParams != "flow=cli" || a.SSO.CallbackPath != "/" {
		t.Errorf("%s asks for %q and expects the redirect at %q, want flow=cli and /",
			tool.Name, a.SSO.ExtraParams, a.SSO.CallbackPath)
	}
}

func checkPassword(t *testing.T, tool manifest.Tool) {
	t.Helper()
	a := tool.Auth
	if a.Password == nil || a.Password.Path == "" {
		t.Fatalf("%s declares a password login with no endpoint", tool.Name)
	}
	if a.Password.TokenField == "" && a.CookieName == "" {
		t.Errorf("%s returns neither a token field nor a cookie name", tool.Name)
	}
}
