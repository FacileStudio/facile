package authflow

import (
	"errors"
	"fmt"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// chooseFlow picks between the flows a tool declares, from the catalog entry,
// what the tool's own discovery endpoint answered, and what the identity
// provider says it can do. Discovery decides when it answered; when it did
// not, the catalog's declared kind stands, because an unreachable /auth/config
// is no reason to refuse to try.
func chooseFlow(a *manifest.Auth, found discovery, session *Session) (string, error) {
	switch a.Kind {
	case "sso":
		return chooseBrowserFlow(a, found)

	case "oidc-device":
		return chooseDeviceFlow(a, found, session)

	case "password":
		if a.Password == nil {
			return "", fmt.Errorf(
				"the catalog declares a password login with no endpoint — report it against the facile catalog")
		}
		if found.answered && !a.Password.WithEmail && !found.passwordNeeded {
			return "passwordless", nil
		}
		return "password", nil

	case "device":
		if a.Device == nil {
			return "", fmt.Errorf("the catalog declares a device login with no endpoints — report it against the facile catalog")
		}
		return "device", nil

	case "token":
		return "token", nil
	}
	return "", fmt.Errorf("unknown login kind %q — report it against the facile catalog", a.Kind)
}

// chooseBrowserFlow settles the question a tool with two credentials asks:
// federate, or use the account the tool holds itself.
func chooseBrowserFlow(a *manifest.Auth, found discovery) (string, error) {
	if found.answered && found.ssoOnly && !found.oidcEnabled {
		return "", fmt.Errorf("the server allows only SSO logins but has no provider configured — ask whoever runs it")
	}
	if a.Password != nil && found.answered && !found.ssoOnly {
		if !found.oidcEnabled {
			return "password", nil
		}
		if !confirm("Sign in with SSO?") {
			return "password", nil
		}
	}
	return "sso", nil
}

// chooseDeviceFlow prefers RFC 8628 and falls back to the loopback flow the
// tool still declares. It asks the browser question first, so a user who wanted
// the tool's own password still gets it, and only upgrades the browser answer:
// the two flows differ in where the browser is, not in who is signing in.
//
// The loopback flow stays the same-machine path and is what runs until the
// provider advertises the grant, which is why moving a tool onto this kind
// changes nothing for anybody on the day it lands.
func chooseDeviceFlow(a *manifest.Auth, found discovery, session *Session) (string, error) {
	if a.OIDCDevice == nil || a.OIDCDevice.Issuer == "" {
		return "", fmt.Errorf("the catalog declares a device sign-in with no issuer — report it against the facile catalog")
	}
	chosen, err := chooseBrowserFlow(a, found)
	if err != nil || chosen != "sso" {
		return chosen, err
	}
	if session.offersDeviceGrant(a.OIDCDevice.Issuer) {
		return "oidc-device", nil
	}
	return "sso", nil
}

// runFlow drives the flow chooseFlow settled on. A passwordless instance is
// not in here on purpose: there is no handshake to run and nothing to store.
func runFlow(kind string, tool manifest.Tool, serverURL string, opts Options) (string, error) {
	a := tool.Auth
	switch kind {
	case "sso":
		return ssoLogin(a, serverURL, opts)
	case "oidc-device":
		token, err := oidcDeviceLogin(a, serverURL, opts)
		if errors.Is(err, errExchangeUnsupported) && a.SSO != nil {
			ui.Hint("this one has not learned the shared sign-in yet — using its own browser login")
			return ssoLogin(a, serverURL, opts)
		}
		return token, err
	case "password":
		return passwordLogin(a, serverURL)
	case "device":
		return deviceLogin(a, serverURL, opts)
	case "token":
		return tokenLogin(tool, serverURL, opts.NoBrowser)
	}
	return "", nil
}
