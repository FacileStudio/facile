package authflow

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

func TestAwaitCallback(t *testing.T) {
	cases := []struct {
		name    string
		flow    manifest.SSOFlow
		request func(base, state string) string
		want    string
		wantErr bool
	}{
		{
			name: "a matching state is accepted",
			flow: manifest.SSOFlow{CallbackWith: "token", CallbackPath: "/callback", RequireState: true},
			request: func(base, state string) string {
				return base + "/callback?token=good&state=" + state
			},
			want: "good",
		},
		{
			name: "a mismatched state is refused",
			flow: manifest.SSOFlow{CallbackWith: "token", CallbackPath: "/callback", RequireState: true},
			request: func(base, _ string) string {
				return base + "/callback?token=stolen&state=not-the-nonce"
			},
			wantErr: true,
		},
		{
			name: "a missing state is refused when one is required",
			flow: manifest.SSOFlow{CallbackWith: "token", CallbackPath: "/callback", RequireState: true},
			request: func(base, _ string) string {
				return base + "/callback?token=stolen"
			},
			wantErr: true,
		},
		{
			name: "sablier's root callback carries a code",
			flow: manifest.SSOFlow{CallbackWith: "code", CallbackPath: "/", RequireState: false},
			request: func(base, _ string) string {
				return base + "/?code=one-time"
			},
			want: "one-time",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			base := "http://" + listener.Addr().String()
			const state = "0123456789abcdef0123456789abcdef"

			type result struct {
				value string
				err   error
			}
			done := make(chan result, 1)
			go func() {
				value, err := awaitCallback(listener, &c.flow, state)
				done <- result{value, err}
			}()

			response, err := http.Get(c.request(base, state))
			if err != nil {
				t.Fatalf("callback request: %v", err)
			}
			response.Body.Close()

			got := <-done
			if c.wantErr {
				if got.err == nil {
					t.Fatalf("a bad callback was accepted and returned %q", got.value)
				}
				if response.StatusCode != http.StatusBadRequest && response.StatusCode != http.StatusNotFound {
					t.Fatalf("browser saw %d, want 400 or 404", response.StatusCode)
				}
				return
			}
			if got.err != nil {
				t.Fatalf("a good callback failed: %v", got.err)
			}
			if got.value != c.want {
				t.Fatalf("callback yielded %q, want %q", got.value, c.want)
			}
		})
	}
}

// TestAwaitCallbackIgnoresStrayRequests covers the browser asking for
// /favicon.ico unprompted: answering it as if it were the redirect would fail
// a login for no reason.
func TestAwaitCallbackIgnoresStrayRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	const state = "0123456789abcdef0123456789abcdef"
	flow := manifest.SSOFlow{CallbackWith: "token", CallbackPath: "/callback", RequireState: true}

	done := make(chan string, 1)
	go func() {
		value, err := awaitCallback(listener, &flow, state)
		if err != nil {
			done <- "error: " + err.Error()
			return
		}
		done <- value
	}()

	stray, err := http.Get(base + "/favicon.ico")
	if err != nil {
		t.Fatal(err)
	}
	stray.Body.Close()
	if stray.StatusCode != http.StatusNotFound {
		t.Fatalf("stray request got %d, want 404", stray.StatusCode)
	}

	good, err := http.Get(base + "/callback?token=good&state=" + state)
	if err != nil {
		t.Fatal(err)
	}
	good.Body.Close()

	if value := <-done; value != "good" {
		t.Fatalf("the listener did not survive a stray request: %q", value)
	}
}

func TestNonceIsHexAndUnique(t *testing.T) {
	first, err := nonce()
	if err != nil {
		t.Fatal(err)
	}
	second, err := nonce()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two nonces were identical")
	}
	if len(first) != 32 {
		t.Fatalf("nonce is %d characters, want 32", len(first))
	}
	for _, r := range first {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("nonce is not hexadecimal: %q", first)
		}
	}
}

func TestCookieScraping(t *testing.T) {
	cases := []struct {
		name   string
		header http.Header
		want   string
	}{
		{
			name:   "antenne's session cookie",
			header: http.Header{"Set-Cookie": []string{"antenne_session=abc123; Path=/; HttpOnly; SameSite=Lax"}},
			want:   "abc123",
		},
		{
			name: "the right cookie among several",
			header: http.Header{"Set-Cookie": []string{
				"csrf=nope; Path=/",
				"antenne_session=abc123; Path=/; Secure",
			}},
			want: "abc123",
		},
		{
			name:   "no cookie at all means no password is configured",
			header: http.Header{},
			want:   "",
		},
		{
			name:   "an emptied cookie is not a credential",
			header: http.Header{"Set-Cookie": []string{"antenne_session=; Max-Age=0"}},
			want:   "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := response{header: c.header}.cookie("antenne_session")
			if got != c.want {
				t.Fatalf("cookie = %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveServerPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stored := filepath.Join(home, ".stored.yml")
	if err := os.WriteFile(stored, []byte("server_url: http://from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	auth := func() *manifest.Auth {
		return &manifest.Auth{
			DefaultServerURL: "https://default.example",
			EnvURL:           "TEST_SERVER_URL",
			Store: &manifest.Store{
				Kind: "file", Path: stored, Format: "yaml", URLField: "server_url",
			},
		}
	}

	cases := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{"the flag wins", "http://from-flag", "http://from-env", "http://from-flag"},
		{"then the environment", "", "http://from-env", "http://from-env"},
		{"then what a previous login stored", "", "", "http://from-file"},
		{"a bare host gets https", "example.test", "", "https://example.test"},
		{"a trailing slash is dropped", "http://x/", "", "http://x"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TEST_SERVER_URL", c.env)
			got, err := resolveServer(auth(), c.flag)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("resolveServer = %q, want %q", got, c.want)
			}
		})
	}

	t.Run("then the catalog default", func(t *testing.T) {
		t.Setenv("TEST_SERVER_URL", "")
		a := auth()
		a.Store.Path = filepath.Join(home, "absent.yml")
		got, err := resolveServer(a, "")
		if err != nil {
			t.Fatal(err)
		}
		if got != "https://default.example" {
			t.Fatalf("resolveServer = %q, want the catalog default", got)
		}
	})
}

func TestStartURLCarriesPortStateAndExtras(t *testing.T) {
	flow := &manifest.SSOFlow{
		StartPath:   "/auth/oidc",
		PortParam:   "port",
		StateParam:  "cli_state",
		ExtraParams: "flow=cli",
	}
	raw, err := startURL("https://casier.example/api", flow, 52345, "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/api/auth/oidc" {
		t.Fatalf("path = %q", parsed.Path)
	}
	want := map[string]string{"port": "52345", "cli_state": "deadbeef", "flow": "cli"}
	for key, value := range want {
		if got := parsed.Query().Get(key); got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
}

func TestChooseFlow(t *testing.T) {
	sso := &manifest.Auth{Kind: "sso", SSO: &manifest.SSOFlow{}}
	ssoWithPassword := &manifest.Auth{Kind: "sso", SSO: &manifest.SSOFlow{}, Password: &manifest.PasswordFlow{Path: "/auth/login", WithEmail: true}}
	password := &manifest.Auth{Kind: "password", Password: &manifest.PasswordFlow{Path: "/api/login"}}

	cases := []struct {
		name    string
		auth    *manifest.Auth
		found   discovery
		want    string
		wantErr bool
	}{
		{"an unreachable discovery keeps the declared kind", sso, discovery{}, "sso", false},
		{"sso_only with no provider is a dead end", sso, discovery{answered: true, ssoOnly: true}, "", true},
		{"no OIDC falls back to the password endpoint", ssoWithPassword, discovery{answered: true}, "password", false},
		{"an instance with no password needs no prompt", password, discovery{answered: true, passwordNeeded: false}, "passwordless", false},
		{"an instance with a password asks for it", password, discovery{answered: true, passwordNeeded: true}, "password", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := chooseFlow(c.auth, c.found)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected a refusal, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
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
		case "sso":
			if a.SSO == nil || a.SSO.StartPath == "" {
				t.Errorf("%s declares an SSO login with no start path", tool.Name)
			} else if a.SSO.CallbackWith == "code" && a.SSO.ExchangePath == "" {
				t.Errorf("%s exchanges a code with no exchange path", tool.Name)
			}
			// A code flow is porte's login-code contract: the parameters it
			// needs are fixed, not per-server taste. Enforcing them here is what
			// keeps a catalog entry from silently drifting back to the stale
			// loopback-token spelling (cli_port / /callback / callbackWith
			// token), which passes a token-less check and times out at login.
			if a.SSO != nil && a.SSO.CallbackWith == "code" {
				if a.SSO.PortParam != "port" {
					t.Errorf("%s: porte reads the port as %q, want \"port\"", tool.Name, a.SSO.PortParam)
				}
				if a.SSO.StateParam != "cli_state" {
					t.Errorf("%s: the nonce goes out as %q, want \"cli_state\"", tool.Name, a.SSO.StateParam)
				}
				if !strings.Contains(a.SSO.ExtraParams, "flow=cli") {
					t.Errorf("%s: the code flow needs extraParams \u0022flow=cli\u0022 (got %q)", tool.Name, a.SSO.ExtraParams)
				}
				if a.SSO.CallbackPath != "/" {
					t.Errorf("%s: porte redirects the loopback to %q, want \"/\"", tool.Name, a.SSO.CallbackPath)
				}
			}
		case "password":
			if a.Password == nil || a.Password.Path == "" {
				t.Errorf("%s declares a password login with no endpoint", tool.Name)
			}
			if a.Password != nil && a.Password.TokenField == "" && a.CookieName == "" {
				t.Errorf("%s returns neither a token field nor a cookie name", tool.Name)
			}
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
	fmt.Fprint(os.Stdout, "")
}
