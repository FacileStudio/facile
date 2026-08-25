package authflow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
)

// fakeProvider is an RFC 8628 provider whose token endpoint answers a scripted
// list of outcomes. It exists because the real provider serves exactly one of
// them: a live endpoint can prove the happy path and nothing else, and the
// error paths are the half of this flow the user actually meets.
type fakeProvider struct {
	discovery     map[string]any
	authorization map[string]any
	authStatus    int
	script        []string

	mu    sync.Mutex
	polls int
	at    []time.Time
}

// newProvider serves discovery, device authorization and the token endpoint.
// grant says whether it advertises the device grant, which is the only signal
// facile uses to decide whether to run this flow at all.
func newProvider(t *testing.T, grant bool, script ...string) (*fakeProvider, string) {
	t.Helper()
	p := &fakeProvider{script: script, authStatus: http.StatusOK}
	server := httptest.NewServer(p)
	t.Cleanup(server.Close)

	grants := []any{"authorization_code", "refresh_token"}
	if grant {
		grants = append(grants, deviceGrantType)
	}
	p.discovery = map[string]any{
		"issuer":                        server.URL,
		"token_endpoint":                server.URL + "/oauth/token",
		"device_authorization_endpoint": server.URL + "/device_authorization",
		"grant_types_supported":         grants,
	}
	p.authorization = map[string]any{
		"device_code":               "device-code-abc",
		"user_code":                 "WDJB-MJHT",
		"verification_uri":          server.URL + "/device",
		"verification_uri_complete": server.URL + "/device?user_code=WDJB-MJHT",
		"expires_in":                600,
		"interval":                  5,
	}
	return p, server.URL
}

func (p *fakeProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case discoveryPath:
		sendJSON(w, http.StatusOK, p.discovery)
	case "/device_authorization":
		sendJSON(w, p.authStatus, p.authorization)
	default:
		p.mu.Lock()
		status, body := pollAnswer(p.script, p.polls)
		p.polls++
		p.at = append(p.at, time.Now())
		p.mu.Unlock()
		sendJSON(w, status, body)
	}
}

// pollAnswer turns a scripted outcome into the wire form RFC 8628 §3.5 gives
// it. Running off the end of the script keeps answering authorization_pending,
// which is what a provider does while nobody has typed the code yet.
func pollAnswer(script []string, n int) (int, map[string]any) {
	outcome := "authorization_pending"
	if n < len(script) {
		outcome = script[n]
	}
	switch outcome {
	case "ok":
		return http.StatusOK, map[string]any{
			"access_token": "provider-access-token", "token_type": "Bearer", "expires_in": 3600,
		}
	case "empty":
		return http.StatusOK, map[string]any{"token_type": "Bearer"}
	}
	return http.StatusBadRequest, map[string]any{
		"error": outcome, "error_description": "scripted " + outcome,
	}
}

func sendJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// fastPoll is the schedule with the sleeps taken out. interval and step are
// fields precisely so this is possible: a test that has to wait five real
// seconds to prove a five second backoff is a test nobody runs.
func fastPoll(interval, step time.Duration) *schedule {
	return &schedule{interval: interval, step: step, deadline: time.Now().Add(5 * time.Second)}
}

func TestDeviceGrantPollsUntilApproved(t *testing.T) {
	provider, issuer := newProvider(t, true, "authorization_pending", "authorization_pending", "ok")
	ep, err := discoverIssuer(issuer)
	if err != nil {
		t.Fatal(err)
	}
	flow := &manifest.OIDCDeviceFlow{Issuer: issuer, ClientID: "facile-cli"}

	auth, err := requestDeviceCode(ep, flow)
	if err != nil {
		t.Fatal(err)
	}
	if auth.userCode != "WDJB-MJHT" || auth.interval != 5*time.Second || auth.expires != 10*time.Minute {
		t.Fatalf("authorization read back as %+v", auth)
	}

	token, err := awaitDeviceToken(ep, flow, auth, fastPoll(time.Millisecond, slowDownStep))
	if err != nil {
		t.Fatal(err)
	}
	if token != "provider-access-token" {
		t.Fatalf("token = %q, want the provider's access token", token)
	}
	if provider.polls != 3 {
		t.Fatalf("%d polls, want 3: two pending then the grant", provider.polls)
	}
}

// slow_down is not advice. A client that keeps its interval is the one the
// provider rate-limits, and the increase is cumulative: two of them are ten
// seconds, not five.
func TestDeviceGrantBacksOffOnSlowDown(t *testing.T) {
	provider, issuer := newProvider(t, true, "slow_down", "slow_down", "ok")
	ep, err := discoverIssuer(issuer)
	if err != nil {
		t.Fatal(err)
	}
	flow := &manifest.OIDCDeviceFlow{Issuer: issuer, ClientID: "facile-cli"}

	const step = 40 * time.Millisecond
	auth := deviceAuth{deviceCode: "device-code-abc", expires: 5 * time.Second}
	if _, err := awaitDeviceToken(ep, flow, auth, fastPoll(5*time.Millisecond, step)); err != nil {
		t.Fatal(err)
	}
	if len(provider.at) != 3 {
		t.Fatalf("%d polls, want 3", len(provider.at))
	}

	first := provider.at[1].Sub(provider.at[0])
	second := provider.at[2].Sub(provider.at[1])
	if first < step || second < first+step {
		t.Fatalf("gaps were %s then %s; each slow_down must add %s to the last interval", first, second, step)
	}
}

// The four RFC 8628 errors are four different things to tell somebody. Saying
// "expired" to a user who refused sends them to retry what they meant to stop.
func TestDeviceGrantReportsEachRefusalDifferently(t *testing.T) {
	cases := []struct {
		outcome string
		want    string
	}{
		{"access_denied", "refused at the provider"},
		{"expired_token", "expired the code before it was approved"},
		{"empty", "returned no access token"},
		{"unauthorized_client", "scripted unauthorized_client"},
	}

	for _, c := range cases {
		t.Run(c.outcome, func(t *testing.T) {
			_, issuer := newProvider(t, true, c.outcome)
			ep, err := discoverIssuer(issuer)
			if err != nil {
				t.Fatal(err)
			}
			auth := deviceAuth{deviceCode: "device-code-abc", expires: 5 * time.Second}
			flow := &manifest.OIDCDeviceFlow{Issuer: issuer, ClientID: "facile-cli"}

			_, err = awaitDeviceToken(ep, flow, auth, fastPoll(time.Millisecond, slowDownStep))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("%s gave %v, want an error saying %q", c.outcome, err, c.want)
			}
		})
	}
}

// expires_in is a promise the client has to keep too: polling past it is
// polling an authorization the provider has already dropped.
func TestDeviceGrantStopsWhenTheCodeExpires(t *testing.T) {
	provider, issuer := newProvider(t, true)
	ep, err := discoverIssuer(issuer)
	if err != nil {
		t.Fatal(err)
	}
	flow := &manifest.OIDCDeviceFlow{Issuer: issuer, ClientID: "facile-cli"}
	auth := deviceAuth{deviceCode: "device-code-abc", expires: 30 * time.Millisecond}
	poll := &schedule{interval: time.Millisecond, step: slowDownStep, deadline: time.Now().Add(auth.expires)}

	_, err = awaitDeviceToken(ep, flow, auth, poll)
	if err == nil || !strings.Contains(err.Error(), "expired after 0s without being approved") {
		t.Fatalf("waiting past expires_in gave %v, want a clean stop", err)
	}
	if provider.polls == 0 {
		t.Fatal("the loop never polled at all")
	}
}
