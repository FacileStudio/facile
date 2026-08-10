package manifest

import "strings"

// Auth describes how a tool authenticates and, crucially, where it expects to
// find its credential afterwards. `facile login` drives the flow and then writes
// the result into that exact location, so the tool itself needs no change to
// benefit. Every field here was read off a real CLI, not invented.
type Auth struct {
	// Kind is the flow facile runs: none, sso, password, device or token.
	// "token" means the credential is minted elsewhere and pasted in.
	Kind string `yaml:"kind"`

	// DefaultServerURL may be empty. A self-hosted appliance is right to refuse
	// to guess an address, so an empty default means facile must ask.
	DefaultServerURL string `yaml:"defaultServerUrl"`

	// APISuffix is appended when the user gives a bare origin. Some CLIs store
	// the API root rather than the site root and 404 on everything without it.
	APISuffix string `yaml:"apiSuffix"`

	// DiscoveryPath returns {sso_only, oidc_enabled} so facile can pick a flow
	// without asking the user what their instance is configured for.
	DiscoveryPath string `yaml:"discoveryPath"`

	SSO      *SSOFlow      `yaml:"sso"`
	Password *PasswordFlow `yaml:"password"`
	Device   *DeviceFlow   `yaml:"device"`

	// IdentityPath is fetched after login purely to name who signed in.
	IdentityPath string `yaml:"identityPath"`

	// Transport is how the credential is later presented: bearer or cookie.
	Transport  string `yaml:"transport"`
	CookieName string `yaml:"cookieName"`

	EnvToken string `yaml:"envToken"`
	EnvURL   string `yaml:"envUrl"`

	Store *Store `yaml:"store"`

	// Note explains a tool that cannot be logged into, so facile can say why
	// instead of pretending the command did something.
	Note string `yaml:"note"`
}

// SSOFlow is the browser round trip. The callback carries either the token
// directly or a one-time code to exchange; the code form is preferred, since a
// token in a query string lands in browser history.
type SSOFlow struct {
	StartPath    string `yaml:"startPath"`
	PortParam    string `yaml:"portParam"`
	StateParam   string `yaml:"stateParam"`
	ExtraParams  string `yaml:"extraParams"`
	CallbackWith string `yaml:"callbackWith"`
	ExchangePath string `yaml:"exchangePath"`

	// CallbackPath is the loopback path the server redirects to, and it is not
	// the same everywhere: casier redirects to /callback, porte — and so
	// sablier — redirects to /. Empty means /callback.
	CallbackPath string `yaml:"callbackPath"`

	// RequireState is false only where the server does not echo the nonce back
	// yet. It is a defect to be fixed server-side, never a setting to prefer.
	RequireState bool `yaml:"requireState"`
}

// PasswordFlow posts credentials to the tool's own API.
type PasswordFlow struct {
	Path string `yaml:"path"`

	// WithEmail distinguishes a multi-user login from a single instance password.
	WithEmail bool `yaml:"withEmail"`

	// TokenField is empty when the credential comes back as a Set-Cookie header.
	TokenField string `yaml:"tokenField"`
}

// DeviceFlow is the headless path: no browser on the machine being authorized.
type DeviceFlow struct {
	StartPath string `yaml:"startPath"`
	PollPath  string `yaml:"pollPath"`
}

// Store is where the credential lands. Getting a single character wrong here
// means facile writes a token the tool will never read, so these values are
// transcribed from each CLI's own read path rather than chosen.
type Store struct {
	Kind string `yaml:"kind"`

	KeychainService string `yaml:"keychainService"`
	KeychainAccount string `yaml:"keychainAccount"`

	Path       string   `yaml:"path"`
	Format     string   `yaml:"format"`
	TokenField string   `yaml:"tokenField"`
	URLField   string   `yaml:"urlField"`
	Mode       uint32   `yaml:"mode"`
	DirMode    uint32   `yaml:"dirMode"`
	Preserve   bool     `yaml:"preserve"`
	Extra      []string `yaml:"extra"`
}

// NeedsLogin reports whether `facile login` can do anything for this tool.
func (t Tool) NeedsLogin() bool {
	return t.Auth != nil && t.Auth.Kind != "" && t.Auth.Kind != "none"
}

// Note is why a tool cannot be logged into, or what is unusual about how it
// stores its credential. Empty when there is nothing to say.
func (t Tool) Note() string {
	if t.Auth == nil {
		return ""
	}
	return strings.TrimSpace(t.Auth.Note)
}

// EnvToken is the environment variable that overrides the stored credential.
// It matters at logout: a variable still set keeps working, and the user would
// otherwise think the logout failed.
func (t Tool) EnvToken() string {
	if t.Auth == nil {
		return ""
	}
	return t.Auth.EnvToken
}
