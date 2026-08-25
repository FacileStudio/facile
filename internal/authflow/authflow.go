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

// Options are the parts of a login the user controls from the command line,
// plus the run they belong to.
type Options struct {
	Server    string
	NoBrowser bool

	// Session is shared by every tool in one `facile login`, so a provider is
	// discovered once and signed in to once. A nil Session still works and
	// simply shares nothing, which is right for a single tool.
	Session *Session
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
//
// A password flow against an instance with no password configured answers 200
// and sets no cookie: every caller is already served as the admin, so an empty
// token there is a success with nothing to store rather than a failure.
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

	kind, err := chooseFlow(a, found, opts.Session, serverURL)
	if err != nil {
		return outcome, err
	}
	if kind == "passwordless" {
		outcome.Passwordless = true
		outcome.Identity = found.username
	}

	if cred.Token, err = runFlow(kind, tool, serverURL, opts); err != nil {
		return outcome, err
	}
	if kind == "password" && cred.Token == "" {
		outcome.Passwordless = true
	}

	return outcome, record(a, cred, &outcome)
}

// record writes the credential where the tool reads it and then names who
// signed in. The identity lookup comes last and cannot fail the login: a run
// that stored a working token succeeded whatever the name lookup answers.
func record(a *manifest.Auth, cred credstore.Credential, outcome *Outcome) error {
	written, err := credstore.Write(a.Store, cred)
	if err != nil {
		return err
	}
	outcome.Locations = written.Locations
	outcome.KeychainFallback = written.KeychainFallback

	if outcome.Identity == "" {
		outcome.Identity = identity(a, cred.ServerURL, cred.Token)
	}
	return nil
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

// extras supplies the values a store asks for by name, such as the machine
// mycelium records alongside its token.
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
