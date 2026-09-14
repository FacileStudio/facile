package authflow

import (
	"fmt"
	"net/http"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// passwordLogin posts to the tool's own login endpoint. The credential comes
// back either in a JSON field or as a Set-Cookie header; which one is a
// manifest fact, since antenne answers with a session cookie and casier with a
// bearer token from the same shape of request.
func passwordLogin(a *manifest.Auth, serverURL string) (string, error) {
	flow := a.Password
	payload, err := passwordPayload(flow)
	if err != nil {
		return "", err
	}

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

// passwordPayload asks for the identity this flow needs. A missing email is a
// dead end worth naming; a blank password is checked by the server, so it is
// passed through rather than guessed at.
func passwordPayload(flow *manifest.PasswordFlow) (map[string]string, error) {
	payload := map[string]string{}
	if flow.WithEmail {
		email, err := ask("Email")
		if err != nil {
			return nil, err
		}
		if email == "" {
			return nil, fmt.Errorf("no email given — run `facile login` again")
		}
		payload["email"] = email
	}
	password, err := askSecret("Password")
	if err != nil {
		return nil, err
	}
	payload["password"] = password
	return payload, nil
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
