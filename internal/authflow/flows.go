package authflow

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// defaultPollInterval is what a device flow uses when the server names none.
const defaultPollInterval = 5 * time.Second

// passwordLogin posts to the tool's own login endpoint. The credential comes
// back either in a JSON field or as a Set-Cookie header; which one is a
// manifest fact, since antenne answers with a session cookie and casier with a
// bearer token from the same shape of request.
func passwordLogin(a *manifest.Auth, serverURL string) (string, error) {
	flow := a.Password
	payload := map[string]string{}

	if flow.WithEmail {
		email, err := ask("Email")
		if err != nil {
			return "", err
		}
		if email == "" {
			return "", fmt.Errorf("no email given — run `facile login` again")
		}
		payload["email"] = email
	}

	password, err := askSecret("Password")
	if err != nil {
		return "", err
	}
	payload["password"] = password

	res, err := post(serverURL+flow.Path, payload)
	if err != nil {
		return "", err
	}
	if res.status == http.StatusUnauthorized || res.status == http.StatusForbidden {
		return "", fmt.Errorf("the server rejected those credentials — check them and run `facile login` again")
	}
	if !res.ok() {
		return "", fmt.Errorf("the server refused the login (%d) — try again", res.status)
	}

	if flow.TokenField != "" {
		token := stringField(res.decode(), flow.TokenField)
		if token == "" {
			return "", fmt.Errorf("the server accepted the login but returned no token — report it against the server")
		}
		return token, nil
	}
	return res.cookie(a.CookieName), nil
}

// deviceLogin is the headless path: the machine being authorized never opens
// the identity provider itself.
func deviceLogin(a *manifest.Auth, serverURL string, opts Options) (string, error) {
	flow := a.Device
	machine, _ := os.Hostname()

	res, err := post(serverURL+flow.StartPath, map[string]string{"machine": machine})
	if err != nil {
		return "", err
	}
	if !res.ok() {
		return "", fmt.Errorf("the server would not start an authorization (%d) — check the server URL", res.status)
	}

	doc := res.decode()
	deviceCode := stringField(doc, "device_code")
	userCode := stringField(doc, "user_code")
	verify := stringField(doc, "verification_uri_complete")
	if deviceCode == "" {
		return "", fmt.Errorf("the server's device authorization was not usable — report it against the server")
	}

	interval := time.Duration(numberField(doc, "interval")) * time.Second
	if interval <= 0 {
		interval = defaultPollInterval
	}
	expires := time.Duration(numberField(doc, "expires_in")) * time.Second
	if expires <= 0 {
		expires = 10 * time.Minute
	}

	ui.Step("Confirm the code %s to authorize this machine", userCode)
	if verify != "" {
		if opts.NoBrowser || !openBrowser(verify) {
			ui.Hint("open %s", verify)
		} else {
			ui.Hint("if nothing opened: %s", verify)
		}
	}
	ui.Step("Waiting for approval")

	deadline := time.Now().Add(expires)
	for time.Now().Before(deadline) {
		time.Sleep(interval)

		res, err := post(serverURL+flow.PollPath, map[string]string{"device_code": deviceCode})
		if err != nil {
			continue
		}
		switch {
		case res.ok():
			token := stringField(res.decode(), "token")
			if token == "" {
				return "", fmt.Errorf("the server approved the machine but returned no token — report it against the server")
			}
			return token, nil
		case res.status == http.StatusBadRequest || res.status == http.StatusForbidden:
			return "", fmt.Errorf("the authorization was denied or expired — run `facile login` again")
		}
	}
	return "", fmt.Errorf("the authorization timed out — run `facile login` again")
}

// tokenLogin covers the tools that mint their credential elsewhere: opus in its
// dashboard, nuage by hand. There is no flow to drive, only a value to accept —
// so the one thing worth doing is opening the page, since "generate a key in
// your dashboard" otherwise means hunting for which page that is.
func tokenLogin(tool manifest.Tool, serverURL string, noBrowser bool) (string, error) {
	if page := tool.TokenPage(serverURL); page != "" {
		if noBrowser || !openBrowser(page) {
			ui.Step("Generate a %s token at %s", tool.Name, page)
		} else {
			ui.Step("Opened %s — generate a token there", page)
		}
	}

	ui.Step("Paste the %s token, it will not be echoed", tool.Name)
	token, err := askSecret("Token")
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("no token given — run `facile login %s` again", tool.Name)
	}
	return token, nil
}

// identity names who signed in. It is cosmetic, so a failure is silence rather
// than an error on an otherwise successful login.
func identity(a *manifest.Auth, serverURL, token string) string {
	if a.IdentityPath == "" || token == "" {
		return ""
	}

	headers := map[string]string{}
	if a.Transport == "cookie" {
		headers["Cookie"] = a.CookieName + "=" + token
	} else {
		headers["Authorization"] = "Bearer " + token
	}

	res, err := get(serverURL+a.IdentityPath, headers)
	if err != nil || !res.ok() {
		return ""
	}
	doc := res.decode()
	for _, key := range []string{"email", "username", "name", "login"} {
		if value := stringField(doc, key); value != "" {
			return value
		}
	}
	return ""
}

func numberField(doc map[string]any, key string) int {
	value, _ := doc[key].(float64)
	return int(value)
}
