// Package authflow runs a tool's login and hands the result to credstore. Which
// flow runs, where it posts and what comes back are all manifest facts, so the
// nine suite CLIs are served by one implementation rather than nine drivers.
package authflow

import (
	"fmt"
	"os"

	"github.com/FacileStudio/facile/internal/credstore"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// Options are the parts of a login the user controls from the command line.
type Options struct {
	Server    string
	NoBrowser bool
}

// Outcome is what the caller reports. It deliberately carries no credential.
type Outcome struct {
	ServerURL string
	Identity  string
	Locations []string

	// KeychainFallback is set when the OS keyring refused the token and it had
	// to go to a file the tool does not read on its own.
	KeychainFallback string

	// Passwordless records an instance that answered "no password needed",
	// which is a success with nothing to store.
	Passwordless bool
}

// Login runs the tool's flow once and writes the credential into the exact
// place the tool's own CLI reads it from.
func Login(tool manifest.Tool, opts Options) (Outcome, error) {
	a := tool.Auth
	if a == nil || !tool.NeedsLogin() {
		return Outcome{}, fmt.Errorf("%s has no login flow — see its note", tool.Name)
	}

	serverURL, err := resolveServer(a, opts.Server)
	if err != nil {
		return Outcome{}, err
	}
	serverURL, found := applySuffix(a, serverURL)

	cred := credstore.Credential{ServerURL: serverURL, Extra: extras()}
	outcome := Outcome{ServerURL: serverURL}

	kind, err := chooseFlow(a, found)
	if err != nil {
		return outcome, err
	}

	switch kind {
	case "passwordless":
		outcome.Passwordless = true
		outcome.Identity = found.username
	case "sso":
		cred.Token, err = ssoLogin(a, serverURL, opts)
	case "password":
		cred.Token, err = passwordLogin(a, serverURL)
	case "device":
		cred.Token, err = deviceLogin(a, serverURL, opts)
	case "token":
		cred.Token, err = tokenLogin(tool)
	}
	if err != nil {
		return outcome, err
	}

	// A password flow against an instance with no password configured answers
	// 200 and sets no cookie: every caller is already served as the admin.
	if kind == "password" && cred.Token == "" {
		outcome.Passwordless = true
	}

	written, err := credstore.Write(a.Store, cred)
	if err != nil {
		return outcome, err
	}
	outcome.Locations = written.Locations
	outcome.KeychainFallback = written.KeychainFallback

	if outcome.Identity == "" {
		outcome.Identity = identity(a, serverURL, cred.Token)
	}
	return outcome, nil
}

// Logout clears the credential and leaves the server URL behind, so the next
// login does not make the user retype where their instance is.
func Logout(tool manifest.Tool) (Outcome, error) {
	a := tool.Auth
	if a == nil || a.Store == nil {
		return Outcome{}, fmt.Errorf("%s stores no credential for facile to clear — see its note", tool.Name)
	}

	serverURL := firstNonEmpty(envOf(a.EnvURL), credstore.StoredServerURL(a.Store), a.DefaultServerURL)
	cleared, err := credstore.Clear(a.Store, serverURL)
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{ServerURL: serverURL, Locations: cleared.Locations}, nil
}

// chooseFlow picks between the flows a tool declares. Discovery decides when it
// answered; when it did not, the catalog's declared kind stands, because an
// unreachable /auth/config is no reason to refuse to try.
func chooseFlow(a *manifest.Auth, found discovery) (string, error) {
	switch a.Kind {
	case "sso":
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

	case "password":
		if a.Password == nil {
			return "", fmt.Errorf("the catalog declares a password login with no endpoint — report it against the facile catalog")
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

// extras supplies the values a store asks for by name, such as the machine
// jardin records alongside its token.
func extras() map[string]string {
	machine, err := os.Hostname()
	if err != nil {
		return nil
	}
	return map[string]string{"machine": machine}
}

// WarnEnvToken tells the user when an environment variable will keep answering
// after a logout, so the clear does not look like it failed.
func WarnEnvToken(a *manifest.Auth) {
	if a == nil || a.EnvToken == "" || os.Getenv(a.EnvToken) == "" {
		return
	}
	ui.Warn("%s is still set in your environment and will keep working", a.EnvToken)
	ui.Hint("unset %s", a.EnvToken)
}
