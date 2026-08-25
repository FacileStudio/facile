package authflow

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
)

// deviceGrantType is the grant identifier from RFC 8628 §3.4, spelled exactly
// as the RFC spells it. A provider that has not been taught this string answers
// unsupported_grant_type, which is how the flow reports itself as not ready.
const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// discoveryPath is the only discovery path worth asking for. Per-application
// paths on this provider answer 200 with the single-page app's HTML, so a
// client that accepts one gets a document with no endpoints in it and no error
// to explain why.
const discoveryPath = "/.well-known/openid-configuration"

// slowDownStep is what RFC 8628 §3.5 requires: on slow_down the client adds
// five seconds to its interval, for this request and every request after it.
// Ignoring it is how a client polls itself into a rate limit and then reports
// the resulting refusal as a failed login.
const slowDownStep = 5 * time.Second

// deviceGrantTimeout bounds the wait when the provider names no expires_in.
const deviceGrantTimeout = 10 * time.Minute

// endpoints is the part of a discovery document facile uses. offersDeviceGrant
// is the provider's own answer to "can you do this at all", and it is the
// switch that lets the catalog declare the flow before the provider serves it.
type endpoints struct {
	deviceAuthorization string
	token               string
	offersDeviceGrant   bool
}

// deviceAuth is the device authorization response, RFC 8628 §3.2. complete is
// verification_uri_complete, which is optional: it carries the user code in the
// URL, so a phone that can follow a link never has to type one.
type deviceAuth struct {
	deviceCode   string
	userCode     string
	verification string
	complete     string
	interval     time.Duration
	expires      time.Duration
}

// schedule is the polling clock. The interval lives here rather than being
// recomputed from the response because slow_down is cumulative: two of them
// mean ten seconds more, not five. step is a field so a test can prove that
// arithmetic without sleeping through it.
type schedule struct {
	interval time.Duration
	step     time.Duration
	deadline time.Time
}

// wait sleeps out one polling interval and reports whether there was still
// time left to poll at all.
func (p *schedule) wait() bool {
	if !time.Now().Before(p.deadline) {
		return false
	}
	time.Sleep(p.interval)
	return true
}

// slower applies RFC 8628 §3.5's slow_down to every request from here on.
func (p *schedule) slower() { p.interval += p.step }

// discoverIssuer reads the endpoints out of the provider's discovery document
// rather than assembling paths from the issuer, so a provider that moves an
// endpoint moves it for facile too and nobody has to ship a release for it.
//
// An advertised endpoint is not an implemented grant: registre serves
// device_authorization_endpoint today and refuses the grant behind it, so
// grant_types_supported is the field that decides.
func discoverIssuer(issuer string) (endpoints, error) {
	res, err := get(strings.TrimSuffix(issuer, "/")+discoveryPath, nil)
	if err != nil {
		return endpoints{}, err
	}
	doc := res.decode()
	if !res.ok() || doc == nil {
		return endpoints{}, fmt.Errorf(
			"%s served no OpenID configuration (%d) — check the issuer in the catalog", issuer, res.status)
	}

	found := endpoints{
		deviceAuthorization: stringField(doc, "device_authorization_endpoint"),
		token:               stringField(doc, "token_endpoint"),
	}
	grants, _ := doc["grant_types_supported"].([]any)
	for _, grant := range grants {
		if text, ok := grant.(string); ok && text == deviceGrantType {
			found.offersDeviceGrant = found.deviceAuthorization != "" && found.token != ""
		}
	}
	return found, nil
}

// postForm sends application/x-www-form-urlencoded, which is what an OAuth
// endpoint takes. Every other flow in facile posts JSON, so this sits beside
// its only callers rather than in http.go pretending to be general.
func postForm(target string, form url.Values) (response, error) {
	request, err := http.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return response{}, err
	}
	return send(request, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "application/json",
	})
}

// requestDeviceCode is RFC 8628 §3.1. It sends no secret: facile is a public
// client, because a CLI on a user's machine has nowhere to keep one.
func requestDeviceCode(ep endpoints, flow *manifest.OIDCDeviceFlow) (deviceAuth, error) {
	res, err := postForm(ep.deviceAuthorization, url.Values{
		"client_id": {flow.ClientID},
		"scope":     {flow.Scopes},
	})
	if err != nil {
		return deviceAuth{}, err
	}

	doc := res.decode()
	if !res.ok() {
		return deviceAuth{}, fmt.Errorf(
			"the provider would not start a device sign-in (%d: %s) — report it against the provider",
			res.status, refusal(doc))
	}

	auth := deviceAuth{
		deviceCode:   stringField(doc, "device_code"),
		userCode:     stringField(doc, "user_code"),
		verification: stringField(doc, "verification_uri"),
		complete:     stringField(doc, "verification_uri_complete"),
		interval:     seconds(numberField(doc, "interval"), defaultPollInterval),
		expires:      seconds(numberField(doc, "expires_in"), deviceGrantTimeout),
	}
	if auth.deviceCode == "" || auth.userCode == "" || auth.verification == "" {
		return deviceAuth{}, fmt.Errorf(
			"the provider's device authorization was not usable — report it against the provider")
	}
	return auth, nil
}

// awaitDeviceToken polls the token endpoint until the user approves, refuses,
// or runs out of time, backing off by five seconds every time the provider
// answers slow_down. A transport error is retried rather than fatal: the
// deadline already bounds the loop, and a dropped packet on the fourth poll is
// no reason to make somebody go and type a new code.
func awaitDeviceToken(ep endpoints, flow *manifest.OIDCDeviceFlow, auth deviceAuth, poll *schedule) (string, error) {
	for poll.wait() {
		res, err := postForm(ep.token, url.Values{
			"grant_type":  {deviceGrantType},
			"device_code": {auth.deviceCode},
			"client_id":   {flow.ClientID},
		})
		if err != nil {
			continue
		}
		token, slower, err := readDeviceToken(res)
		switch {
		case err != nil:
			return "", err
		case token != "":
			return token, nil
		case slower:
			poll.slower()
		}
	}
	return "", fmt.Errorf("the code expired after %s without being approved — run `facile login` again",
		auth.expires.Round(time.Second))
}

// readDeviceToken turns one poll into the three things that can happen: the
// grant completed, keep waiting, or stop. RFC 8628 §3.5's four errors are not
// interchangeable — telling somebody their code expired when they in fact
// refused it sends them to retry the thing they meant to stop.
func readDeviceToken(res response) (string, bool, error) {
	doc := res.decode()
	if res.ok() {
		token := stringField(doc, "access_token")
		if token == "" {
			return "", false, fmt.Errorf(
				"the provider approved this machine but returned no access token — report it against the provider")
		}
		return token, false, nil
	}

	switch stringField(doc, "error") {
	case "authorization_pending":
		return "", false, nil
	case "slow_down":
		return "", true, nil
	case "access_denied":
		return "", false, fmt.Errorf(
			"the sign-in was refused at the provider — run `facile login` again if that was not deliberate")
	case "expired_token":
		return "", false, fmt.Errorf(
			"the provider expired the code before it was approved — run `facile login` again")
	}
	return "", false, fmt.Errorf("the provider refused the device grant (%d: %s) — report it against the provider",
		res.status, refusal(doc))
}

// refusal is the most useful sentence an OAuth error body carries. RFC 6749
// §5.2 makes error_description optional, so this falls through to the code
// itself rather than printing an empty reason.
func refusal(doc map[string]any) string {
	return firstNonEmpty(
		stringField(doc, "error_description"),
		stringField(doc, "error"),
		"no reason given",
	)
}
