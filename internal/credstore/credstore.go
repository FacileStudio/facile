// Package credstore writes a credential into the exact place a tool's own CLI
// already reads it from, and clears it again. Everything it needs comes from
// the manifest's Store block, so nine CLIs are served by one implementation and
// a tenth costs a few lines of YAML.
package credstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
)

// Credential is the result of a login. It is passed by value and never logged:
// only the locations it was written to are ever reported to the user.
type Credential struct {
	Token     string
	ServerURL string

	// Extra carries values a store asks for by name, such as jardin's machine.
	Extra map[string]string
}

// Result names where the credential landed so the caller can say so without
// ever holding the secret again.
type Result struct {
	Locations []string

	// KeychainFallback is set when the OS keyring refused and the token had to
	// go to a file the tool will not read on its own. It is a degraded state
	// the caller must surface, not an error.
	KeychainFallback string
}

// Write puts the credential wherever the store says the tool will look for it.
// A split store — casier keeps the token in the keychain and the server URL in
// a config file — performs both writes or fails.
func Write(s *manifest.Store, cred Credential) (Result, error) {
	if s == nil {
		return Result{}, fmt.Errorf("the catalog gives this tool no credential store — report it against the facile catalog")
	}

	switch s.Kind {
	case "file":
		path, err := writeFile(s, fileFields(s, cred), nil)
		if err != nil {
			return Result{}, err
		}
		return Result{Locations: []string{store.Tilde(path)}}, nil

	case "keychain":
		return writeKeychain(s, cred)

	default:
		return Result{}, fmt.Errorf("unknown credential store kind %q — report it against the facile catalog", s.Kind)
	}
}

// Clear removes the credential and deliberately leaves the server URL behind,
// so the next login does not make the user retype where their instance is.
func Clear(s *manifest.Store, serverURL string) (Result, error) {
	if s == nil {
		return Result{}, fmt.Errorf("the catalog gives this tool no credential store — report it against the facile catalog")
	}

	switch s.Kind {
	case "file":
		if s.TokenField == "" {
			return Result{}, nil
		}
		path, err := writeFile(s, nil, []string{s.TokenField})
		if err != nil {
			return Result{}, err
		}
		return Result{Locations: []string{store.Tilde(path)}}, nil

	case "keychain":
		return clearKeychain(s, serverURL)

	default:
		return Result{}, fmt.Errorf("unknown credential store kind %q — report it against the facile catalog", s.Kind)
	}
}

// StoredServerURL reports the server URL a previous login left behind, so the
// resolution chain can prefer it over a default nobody asked for.
func StoredServerURL(s *manifest.Store) string {
	if s == nil || s.Path == "" || s.URLField == "" {
		return ""
	}
	path, err := Expand(s.Path)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	value, err := read(s.Format, raw, s.URLField)
	if err != nil {
		return ""
	}
	return value
}

// Expand resolves the path templates the catalog uses. ${xdgConfig} and
// ${userConfig} are deliberately different: antenne hardcodes ~/.config/antenne
// while casier uses the platform-native directory that Rust's dirs::config_dir
// returns, and confusing the two writes to a path nobody reads.
func Expand(raw string) (string, error) {
	switch {
	case strings.HasPrefix(raw, "${xdgConfig}"):
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot locate your home directory — set HOME")
			}
			base = filepath.Join(home, ".config")
		}
		return filepath.Join(base, strings.TrimPrefix(raw, "${xdgConfig}")), nil

	case strings.HasPrefix(raw, "${userConfig}"):
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("cannot locate your config directory — set XDG_CONFIG_HOME")
		}
		return filepath.Join(base, strings.TrimPrefix(raw, "${userConfig}")), nil

	case raw == "~" || strings.HasPrefix(raw, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot locate your home directory — set HOME")
		}
		return filepath.Join(home, strings.TrimPrefix(raw, "~")), nil
	}
	return raw, nil
}

// field is one key/value pair to set, kept as a slice so a file created from
// nothing comes out in catalog order rather than map order.
type field struct {
	key   string
	value string
}

func fileFields(s *manifest.Store, cred Credential) []field {
	var fields []field
	if s.URLField != "" && cred.ServerURL != "" {
		fields = append(fields, field{s.URLField, cred.ServerURL})
	}
	if s.TokenField != "" && cred.Token != "" {
		fields = append(fields, field{s.TokenField, cred.Token})
	}
	for _, name := range s.Extra {
		if value, ok := cred.Extra[name]; ok && value != "" {
			fields = append(fields, field{name, value})
		}
	}
	return fields
}

func writeKeychain(s *manifest.Store, cred Credential) (Result, error) {
	var result Result

	// The URL half of a split store goes first: if the keychain then refuses,
	// the user still has a configured server and only a token to sort out.
	if s.Path != "" {
		path, err := writeFile(s, fileFields(&manifest.Store{
			URLField: s.URLField,
			Extra:    s.Extra,
		}, cred), nil)
		if err != nil {
			return result, err
		}
		result.Locations = append(result.Locations, store.Tilde(path))
	}

	acct := keychainAccount(s, cred.ServerURL)
	if err := setKeychain(s.KeychainService, acct, cred.Token); err != nil {
		path, ferr := writeFallback(s, cred.Token)
		if ferr != nil {
			return result, fmt.Errorf("cannot reach your keychain and cannot write a fallback file — %s", ferr)
		}
		result.KeychainFallback = store.Tilde(path)
		return result, nil
	}
	result.Locations = append(result.Locations, "your keychain")
	return result, nil
}

// setKeychain verifies by reading back, because a keyring that accepts a write
// and returns something else is a silent 401 an hour later.
func setKeychain(service, account, token string) error {
	if err := keyring.Set(service, account, token); err != nil {
		return err
	}
	stored, err := keyring.Get(service, account)
	if err != nil {
		return err
	}
	if stored != token {
		return fmt.Errorf("the keychain returned a different value than was written")
	}
	return nil
}

func clearKeychain(s *manifest.Store, serverURL string) (Result, error) {
	var result Result

	acct := keychainAccount(s, serverURL)
	if acct == "" {
		return result, fmt.Errorf("no server URL to identify the keychain entry — run `facile login` first")
	}
	if err := keyring.Delete(s.KeychainService, acct); err == nil {
		result.Locations = append(result.Locations, "your keychain")
	} else if err != keyring.ErrNotFound {
		return result, fmt.Errorf("cannot remove the keychain entry — unlock your keychain and try again")
	}

	path, err := fallbackPath(s)
	if err == nil {
		if err := os.Remove(path); err == nil {
			result.Locations = append(result.Locations, store.Tilde(path))
		}
	}
	return result, nil
}

// keychainAccount resolves the account string byte-for-byte as the tool's CLI
// will compute it at read time. casier's entry is keyed on the server URL
// including its /api suffix; a near-miss stores a token the CLI cannot see.
func keychainAccount(s *manifest.Store, serverURL string) string {
	if s.KeychainAccount == "serverUrl" {
		return serverURL
	}
	return s.KeychainAccount
}

// writeFallback keeps the credential when no secret service exists, which is
// the normal state of a headless Linux box. Refusing outright, as casier does
// today, only leaves the user with nothing.
func writeFallback(s *manifest.Store, token string) (string, error) {
	path, err := fallbackPath(s)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := createAt(path, []byte(token+"\n"), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func fallbackPath(s *manifest.Store) (string, error) {
	if s.Path != "" {
		path, err := Expand(s.Path)
		if err != nil {
			return "", err
		}
		return filepath.Join(filepath.Dir(path), "token"), nil
	}
	return filepath.Join(store.ConfigDir(), s.KeychainService+".token"), nil
}
