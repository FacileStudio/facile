package authflow

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// ssoTimeout is long enough to type a password and answer a second factor,
// short enough that a closed tab does not leave a listener open forever.
const ssoTimeout = 300 * time.Second

// defaultCallbackPath is where casier's server redirects. Sablier's redirects
// to "/" instead, which is why the path is a manifest field and not a constant.
const defaultCallbackPath = "/callback"

func ssoLogin(a *manifest.Auth, serverURL string, opts Options) (string, error) {
	flow := a.SSO
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("cannot open a loopback port to receive the login — check your firewall")
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	state, err := nonce()
	if err != nil {
		return "", err
	}
	if !flow.RequireState {
		ui.Warn("this server does not echo a login nonce back, so the callback cannot be validated")
	}

	target, err := startURL(serverURL, flow, port, state)
	if err != nil {
		return "", err
	}
	openSignIn(target, opts.NoBrowser)

	value, err := awaitCallback(listener, flow, state)
	if err != nil {
		return "", err
	}
	if flow.CallbackWith == "code" {
		return exchange(serverURL+flow.ExchangePath, value)
	}
	return value, nil
}

// openSignIn opens the sign-in page, or prints its URL when the desktop is not
// available to open one.
func openSignIn(target string, noBrowser bool) {
	if noBrowser || !openBrowser(target) {
		ui.Step("Open this URL to sign in")
		ui.Hint("%s", target)
		return
	}
	ui.Step("Opening your browser to sign in")
	ui.Hint("if nothing opened: %s", target)
}

func startURL(serverURL string, flow *manifest.SSOFlow, port int, state string) (string, error) {
	target, err := url.Parse(serverURL + flow.StartPath)
	if err != nil {
		return "", fmt.Errorf("the server URL is not usable — pass --server <url>")
	}

	query := target.Query()
	for pair := range strings.SplitSeq(flow.ExtraParams, "&") {
		if key, value, ok := strings.Cut(pair, "="); ok {
			query.Set(key, value)
		}
	}
	if flow.PortParam != "" {
		query.Set(flow.PortParam, strconv.Itoa(port))
	}
	if flow.StateParam != "" {
		query.Set(flow.StateParam, state)
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

// awaitCallback serves exactly one login. Anything that is not the redirect —
// a browser asking for /favicon.ico unprompted — gets a 404 and the listener
// keeps waiting, because failing that request would fail the login for no
// reason. A state that does not match is the opposite: a hard abort.
func awaitCallback(listener net.Listener, flow *manifest.SSOFlow, state string) (string, error) {
	param := "token"
	if flow.CallbackWith == "code" {
		param = "code"
	}
	path := flow.CallbackPath
	if path == "" {
		path = defaultCallbackPath
	}

	done := make(chan callbackOutcome, 1)
	server := &http.Server{Handler: makeCallbackHandler(path, param, flow.RequireState, state, done)}
	go server.Serve(listener)
	defer server.Close()

	ui.Step("Waiting for the browser to complete sign-in")
	select {
	case result := <-done:
		return result.value, result.err
	case <-time.After(ssoTimeout):
		return "", fmt.Errorf("timed out waiting for the browser — run `facile login` again")
	}
}

type callbackOutcome struct {
	value string
	err   error
}

// makeCallbackHandler answers the one login redirect. Anything else — a browser
// asking for /favicon.ico unprompted — gets a 404 and the listener keeps
// waiting, because failing that request would fail the login for no reason. A
// state that does not match is the opposite: a hard abort.
func makeCallbackHandler(path, param string, requireState bool, state string, done chan callbackOutcome) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			page(w, http.StatusNotFound, "Not the login redirect.")
			return
		}
		value := r.URL.Query().Get(param)
		if value == "" {
			page(w, http.StatusNotFound, "Not the login redirect.")
			return
		}
		if requireState && r.URL.Query().Get("state") != state {
			page(w, http.StatusBadRequest, "The callback did not match this login attempt. Run the command again.")
			done <- callbackOutcome{err: fmt.Errorf("the sign-in callback did not match this login attempt — run `facile login` again")}
			return
		}
		page(w, http.StatusOK, "Signed in. You can close this tab and return to your terminal.")
		done <- callbackOutcome{value: value}
	})
}

func page(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, "<!doctype html><meta charset=\"utf-8\"><title>Facile</title>"+
		"<body style=\"font:16px/1.5 system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem\">"+
		"<h1>Facile</h1><p>%s</p>", message)
}

// exchange trades the one-time code for the real credential. The code form
// exists so the token never travels in a query string, where it would land in
// the user's browser history.
func exchange(url, code string) (string, error) {
	res, err := post(url, map[string]string{"code": code})
	if err != nil {
		return "", err
	}
	if !res.ok() {
		return "", fmt.Errorf("the server refused the login code (%d) — run `facile login` again", res.status)
	}
	token := stringField(res.decode(), "token")
	if token == "" {
		return "", fmt.Errorf("the server returned no token for the login code — report it against the server")
	}
	return token, nil
}

// nonce is 32 hex characters of crypto/rand, the same width casier uses.
func nonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("cannot generate a login nonce — retry")
	}
	return hex.EncodeToString(raw), nil
}
