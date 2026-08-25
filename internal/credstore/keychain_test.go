package credstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/FacileStudio/facile/internal/manifest"
)

// casierStore is transcribed from casier-cli's own read path: the token in the
// OS keychain keyed on the server URL including its /api suffix, the URL in a
// TOML file beside it.
func casierStore(dir string) *manifest.Store {
	return &manifest.Store{
		Kind:            "keychain",
		KeychainService: "casier",
		KeychainAccount: "serverUrl",
		Path:            filepath.Join(dir, "casier", "config.toml"),
		Format:          "toml",
		URLField:        "server_url",
		Mode:            0o644,
	}
}

// A headless Linux box has no secret service, and it is exactly the machine the
// device sign-in exists for: the browser is somewhere else. The token must not
// evaporate there. It goes to a 0600 file and the caller is told, because
// casier-cli will not read that file — the user has to export CASIER_TOKEN — and
// a login that silently stored nothing is worse than one that refused.
func TestAKeychainlessBoxKeepsTheTokenAndSaysWhere(t *testing.T) {
	keyring.MockInitWithError(errors.New("dbus: couldn't determine address of session bus"))
	t.Cleanup(keyring.MockInit)

	dir := t.TempDir()
	store := casierStore(dir)
	cred := Credential{Token: "a-real-token", ServerURL: "https://casier.facile.studio/api"}

	result, err := Write(store, cred)
	if err != nil {
		t.Fatalf("a missing keychain must not fail the login: %v", err)
	}
	if result.KeychainFallback == "" {
		t.Fatal("the fallback went unreported, so the user would never know to export the token")
	}

	fallback := filepath.Join(dir, "casier", "token")
	raw, err := os.ReadFile(fallback)
	if err != nil {
		t.Fatalf("no token at %s: %v", fallback, err)
	}
	if strings.TrimSpace(string(raw)) != "a-real-token" {
		t.Fatalf("the fallback file holds %q", strings.TrimSpace(string(raw)))
	}

	info, err := os.Stat(fallback)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("the fallback is mode %v, want 0600", info.Mode().Perm())
	}
}

// The server URL is written before the keychain is touched, so a box with no
// secret service still ends up configured and only the token needs sorting out.
func TestAKeychainlessBoxStillRecordsTheServer(t *testing.T) {
	keyring.MockInitWithError(errors.New("no secret service"))
	t.Cleanup(keyring.MockInit)

	dir := t.TempDir()
	store := casierStore(dir)
	const serverURL = "https://casier.facile.studio/api"

	if _, err := Write(store, Credential{Token: "t", ServerURL: serverURL}); err != nil {
		t.Fatal(err)
	}
	if got := StoredServerURL(store); got != serverURL {
		t.Fatalf("StoredServerURL = %q, want %q", got, serverURL)
	}
}
