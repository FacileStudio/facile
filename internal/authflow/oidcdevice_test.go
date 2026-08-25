package authflow

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

// fakeApp is the tool's own server: the half that turns the provider's access
// token into the credential the tool's CLI reads. It records what facile sent,
// because sending the wrong field is a login that succeeds and then 401s.
type fakeApp struct {
	status int
	got    map[string]string
}

func (a *fakeApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &a.got)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(a.status)
	if a.status == http.StatusOK {
		io.WriteString(w, `{"user_id":"7","token":"the-tools-own-token"}`)
	}
}

func newApp(t *testing.T, status int) (*fakeApp, string) {
	t.Helper()
	app := &fakeApp{status: status}
	server := httptest.NewServer(app)
	t.Cleanup(server.Close)
	return app, server.URL
}

// The provider decides whether this flow runs, and it decides by the one field
// that cannot be wrong about it. Registre advertises the endpoint today and
// refuses the grant behind it, so an advertised endpoint proves nothing.
func TestDiscoveryDecidesWhetherTheGrantRuns(t *testing.T) {
	_, offering := newProvider(t, true)
	_, withholding := newProvider(t, false)

	if !NewSession().offersDeviceGrant(offering) {
		t.Error("a provider listing the device grant must be used")
	}
	if NewSession().offersDeviceGrant(withholding) {
		t.Error("an advertised endpoint is not an implemented grant")
	}
	if NewSession().offersDeviceGrant("http://127.0.0.1:1") {
		t.Error("an unreachable provider must fall back, not claim the grant")
	}
}

// The whole point, end to end. What lands in the store is the tool's own
// credential rather than the provider's token, and three tools sharing a
// provider ask for one code between them: six codes would not be one login.
func TestDeviceSignInStoresTheToolsOwnToken(t *testing.T) {
	provider, issuer := newProvider(t, true, "ok")
	provider.authorization["interval"] = 1
	app, serverURL := newApp(t, http.StatusOK)
	opts := Options{NoBrowser: true, Session: NewSession()}

	auth := &manifest.Auth{Kind: "oidc-device", Flows: manifest.Flows{
		OIDCDevice: &manifest.OIDCDeviceFlow{
			Issuer: issuer, ClientID: "facile-cli", Scopes: "openid profile email",
			ExchangePath: "/auth/oidc/device/exchange",
		},
	}}

	for _, tool := range []string{"casier", "journal", "courrier"} {
		token, err := oidcDeviceLogin(auth, serverURL, opts)
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		if token != "the-tools-own-token" {
			t.Fatalf("%s stored %q, want the token it reads, not the provider's", tool, token)
		}
	}
	if app.got["access_token"] != "provider-access-token" {
		t.Fatalf("the app was sent %v, want the provider's access token", app.got)
	}
	if provider.polls != 1 {
		t.Fatalf("%d polls for three tools, want 1: the grant is shared", provider.polls)
	}
}

// The exchange endpoint is the piece the apps have yet to ship. A 404 there is
// expected for now and has to read as "not yet", not as a refused sign-in.
//
// The session is seeded rather than granted: this is about the second half of
// the flow, and nobody should wait out a polling interval to reach it.
func TestDeviceSignInSaysSoWhenTheServerHasNoExchangeYet(t *testing.T) {
	_, issuer := newProvider(t, true, "ok")
	_, serverURL := newApp(t, http.StatusNotFound)

	auth := &manifest.Auth{Kind: "oidc-device", Flows: manifest.Flows{
		OIDCDevice: &manifest.OIDCDeviceFlow{
			Issuer: issuer, ClientID: "facile-cli", ExchangePath: "/auth/oidc/device/exchange",
		},
	}}
	session := &Session{tokens: map[string]string{issuer: "provider-access-token"}}

	_, err := oidcDeviceLogin(auth, serverURL, Options{NoBrowser: true, Session: session})
	if err == nil || !strings.Contains(err.Error(), "does not accept a device sign-in yet") {
		t.Fatalf("a 404 exchange gave %v, want it named as not yet available", err)
	}
}

// This is what registre answers today, and it must arrive as the provider's
// own sentence rather than as a bare status code somebody has to go and decode.
func TestAnUnregisteredClientIsReportedInTheProvidersWords(t *testing.T) {
	provider, issuer := newProvider(t, true)
	provider.authStatus = http.StatusInternalServerError
	provider.authorization = map[string]any{
		"error":             "unauthorized_client",
		"error_description": "client missing grant type urn:ietf:params:oauth:grant-type:device_code",
	}

	ep, err := discoverIssuer(issuer)
	if err != nil {
		t.Fatal(err)
	}
	_, err = requestDeviceCode(ep, manifestFlow(issuer))
	if err == nil || !strings.Contains(err.Error(), "client missing grant type") {
		t.Fatalf("an unregistered client gave %v, want the provider's own words", err)
	}
}

// A provider that answers 200 with a body missing the pieces the flow needs is
// a different failure, and printing a blank code at somebody is worse than
// saying the authorization was unusable.
func TestAnUnusableAuthorizationIsRefused(t *testing.T) {
	provider, issuer := newProvider(t, true)
	provider.authorization = map[string]any{"device_code": "abc", "expires_in": 600}

	ep, err := discoverIssuer(issuer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requestDeviceCode(ep, manifestFlow(issuer)); err == nil ||
		!strings.Contains(err.Error(), "device authorization was not usable") {
		t.Fatalf("a code-less authorization gave %v", err)
	}
}

func manifestFlow(issuer string) *manifest.OIDCDeviceFlow {
	return &manifest.OIDCDeviceFlow{Issuer: issuer, ClientID: "facile-cli", Scopes: "openid profile email"}
}
