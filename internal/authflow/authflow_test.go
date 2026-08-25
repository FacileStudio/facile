package authflow

import (
	"net"
	"net/http"
	"net/url"
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
