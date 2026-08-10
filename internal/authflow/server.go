package authflow

import (
	"fmt"
	"strings"

	"github.com/FacileStudio/facile/internal/credstore"
	"github.com/FacileStudio/facile/internal/manifest"
)

// discovery is the union of what the suite's instances answer when asked how
// they authenticate. casier returns {sso_only, oidc_enabled}, antenne returns
// {authenticated, passwordRequired, username} from a different path. Absent
// fields are false, and answered records whether anybody replied at all: a
// discovery failure means "ask the user", not "give up".
type discovery struct {
	answered        bool
	ssoOnly         bool
	oidcEnabled     bool
	passwordNeeded  bool
	alreadySignedIn bool
	username        string
}

func discover(a *manifest.Auth, serverURL string) discovery {
	if a.DiscoveryPath == "" {
		return discovery{}
	}
	res, err := get(serverURL+a.DiscoveryPath, nil)
	if err != nil || !res.ok() {
		return discovery{}
	}
	doc := res.decode()
	if doc == nil {
		return discovery{}
	}
	return discovery{
		answered:        true,
		ssoOnly:         boolField(doc, "sso_only"),
		oidcEnabled:     boolField(doc, "oidc_enabled"),
		passwordNeeded:  boolField(doc, "passwordRequired"),
		alreadySignedIn: boolField(doc, "authenticated"),
		username:        stringField(doc, "username"),
	}
}

// resolveServer applies the precedence a user expects: what they just typed,
// then their environment, then what a previous login left behind, then the
// catalog's default, and only then a prompt. An empty default is legitimate —
// a self-hosted appliance is right to refuse to guess an address.
func resolveServer(a *manifest.Auth, flag string) (string, error) {
	if raw := firstNonEmpty(flag, envOf(a.EnvURL), credstore.StoredServerURL(a.Store), a.DefaultServerURL); raw != "" {
		return normalize(raw)
	}

	raw, err := ask("Server URL")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("no server URL given — pass --server <url>")
	}
	return normalize(raw)
}

// applySuffix settles which of the bare origin and the suffixed one the tool
// should store. With a discovery endpoint, whichever answers wins and is
// persisted; without one there is nothing to probe, so the suffix is simply
// appended the way sablier's own CLI does it.
func applySuffix(a *manifest.Auth, serverURL string) (string, discovery) {
	if a.APISuffix == "" || strings.HasSuffix(serverURL, a.APISuffix) {
		return serverURL, discover(a, serverURL)
	}
	if a.DiscoveryPath == "" {
		return serverURL + a.APISuffix, discovery{}
	}

	for _, candidate := range []string{serverURL, serverURL + a.APISuffix} {
		if found := discover(a, candidate); found.answered {
			return candidate, found
		}
	}
	return serverURL + a.APISuffix, discovery{}
}

// normalize matches what every suite CLI does to a typed-in address: drop the
// trailing slash and assume https when no scheme was given.
func normalize(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", fmt.Errorf("the server URL is empty — pass --server <url>")
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed, nil
	}
	return "https://" + trimmed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
