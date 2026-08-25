package authflow

import (
	"fmt"
	"net/http"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// Session is what one `facile login` run learns once and reuses for every tool
// in it. Without it the premise fails: six tools federating to one provider
// would ask for six codes, and typing six codes is not one login. A nil Session
// is the single-tool case and every method here tolerates it.
type Session struct {
	issuers map[string]endpoints
	tokens  map[string]string
}

// NewSession starts a run. Both caches are keyed by issuer, not by tool,
// because sharing the sign-in is the entire point of sharing a provider.
func NewSession() *Session {
	return &Session{issuers: map[string]endpoints{}, tokens: map[string]string{}}
}

// endpointsFor reads the provider's discovery document once per run. Six tools
// naming the same issuer make one request, not six.
func (s *Session) endpointsFor(issuer string) (endpoints, error) {
	if s != nil {
		if found, ok := s.issuers[issuer]; ok {
			return found, nil
		}
	}
	found, err := discoverIssuer(issuer)
	if err != nil {
		return endpoints{}, err
	}
	if s != nil {
		s.issuers[issuer] = found
	}
	return found, nil
}

// offersDeviceGrant asks the provider whether it implements RFC 8628 at all.
// A provider that does not advertise the grant sends the tool back to the
// loopback browser flow rather than failing the login, which is what lets the
// catalog name the device flow before the provider serves it: no flag day, and
// the flow turns itself on the day grant_types_supported grows the entry.
//
// A provider facile cannot reach answers no for the same reason. Falling back
// to a flow that works beats refusing over a discovery document.
func (s *Session) offersDeviceGrant(issuer string) bool {
	found, err := s.endpointsFor(issuer)
	return err == nil && found.offersDeviceGrant
}

// accessToken runs the grant, or hands back the one this run already has. The
// token is the provider's, not the tool's, and it never reaches disk: it is
// traded for the tool's own credential and forgotten when the process ends.
func (s *Session) accessToken(flow *manifest.OIDCDeviceFlow, opts Options) (string, error) {
	if s != nil {
		if token, ok := s.tokens[flow.Issuer]; ok {
			return token, nil
		}
	}

	ep, err := s.endpointsFor(flow.Issuer)
	if err != nil {
		return "", err
	}
	auth, err := requestDeviceCode(ep, flow)
	if err != nil {
		return "", err
	}
	announceCode(auth, opts)

	poll := &schedule{interval: auth.interval, step: slowDownStep, deadline: time.Now().Add(auth.expires)}
	token, err := awaitDeviceToken(ep, flow, auth, poll)
	if err != nil {
		return "", err
	}
	if s != nil {
		s.tokens[flow.Issuer] = token
	}
	return token, nil
}

// announceCode is the human half of the flow, and the half that decides whether
// any of this works. The code has to survive being read off one screen and
// typed on another device, so it goes on a line of its own, padded, and
// verbatim: the provider chooses its shape and inserting a friendlier dash
// would offer a code the provider will not accept.
//
// There is no QR code. It would be a dependency plus a pile of fallback for
// every terminal that cannot draw one, and verification_uri_complete already
// removes the typing for anyone whose phone can follow a link.
//
// The browser is still opened here in case this machine has one. It usually
// does not, which is why the URL is printed either way rather than only when
// the open fails.
func announceCode(auth deviceAuth, opts Options) {
	ui.Step("Open this page on any device — a phone is fine")
	ui.Hint("%s", auth.verification)
	ui.Step("and enter this code")
	ui.Out("")
	ui.Out("    %s", ui.Accent(auth.userCode))
	ui.Out("")
	if auth.complete != "" {
		ui.Hint("or open %s, which fills it in for you", auth.complete)
	}
	if !opts.NoBrowser && openBrowser(firstNonEmpty(auth.complete, auth.verification)) {
		ui.Hint("a browser opened here too, in case this machine has one")
	}
	ui.Step("Waiting for approval — the code lasts %s", auth.expires.Round(time.Second))
}

// oidcDeviceLogin signs in at the provider once and trades the result for this
// tool's own credential.
func oidcDeviceLogin(a *manifest.Auth, serverURL string, opts Options) (string, error) {
	flow := a.OIDCDevice
	if flow.ExchangePath == "" {
		return "", fmt.Errorf(
			"the catalog declares a device sign-in with no exchange path — report it against the facile catalog")
	}
	token, err := opts.Session.accessToken(flow, opts)
	if err != nil {
		return "", err
	}
	return exchangeDeviceToken(serverURL+flow.ExchangePath, token)
}

// exchangeDeviceToken trades the provider's access token for the tool's own
// credential, for the same reason the loopback flow exchanges a code rather
// than storing it: what the tool reads is its own session, and writing the
// provider's token into that slot is a login that 401s an hour later.
//
// A 404 is the expected answer from a server that has not shipped this
// endpoint yet, so it says so instead of reporting a refused sign-in.
func exchangeDeviceToken(target, accessToken string) (string, error) {
	res, err := post(target, map[string]string{"access_token": accessToken})
	if err != nil {
		return "", err
	}

	switch {
	case res.status == http.StatusNotFound:
		return "", fmt.Errorf("this server does not accept a device sign-in yet (404 on %s) — "+
			"sign in with a browser on this machine, or ask whoever runs it to update it", target)
	case !res.ok():
		return "", fmt.Errorf("the server refused the provider's token (%d) — run `facile login` again", res.status)
	}

	token := stringField(res.decode(), "token")
	if token == "" {
		return "", fmt.Errorf("the server accepted the sign-in but returned no token — report it against the server")
	}
	return token, nil
}
