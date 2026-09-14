package credstore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
)

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

// keychainOrFallback writes the token to the keychain, and on a refused keychain
// to a fallback file so a headless box still keeps the credential. It returns
// the fallback path when that is where the token went, or an error only when
// neither destination worked.
func keychainOrFallback(s *manifest.Store, serverURL, token string) (string, error) {
	acct := keychainAccount(s, serverURL)
	if err := setKeychain(s.KeychainService, acct, token); err == nil {
		return "", nil
	}
	path, ferr := writeFallback(s, token)
	if ferr != nil {
		return "", fmt.Errorf("cannot reach your keychain and cannot write a fallback file — %s", ferr)
	}
	return path, nil
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
