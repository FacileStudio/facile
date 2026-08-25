package manifest

import "strings"

// Auth describes how a tool authenticates and, crucially, where it expects to
// find its credential afterwards. `facile login` drives the flow and then writes
// the result into that exact location, so the tool itself needs no change to
// benefit. Every field here was read off a real CLI, not invented.
type Auth struct {
	// Kind is the flow facile runs: none, sso, oidc-device, password, device
	// or token. "token" means the credential is minted elsewhere and pasted
	// in. "device" is a tool's own headless endpoints; "oidc-device" is the
	// RFC 8628 grant at the shared identity provider, and the two are not the
	// same protocol — the prefix is there so the difference is visible here.
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

	// Flows and Env are grouped rather than listed so this struct stays under
	// filet's field cap as kinds are added. Both are inlined: the YAML is flat
	// and unchanged, and Go promotes the fields, so a.SSO and a.EnvToken still
	// read the way they always did.
	Flows `yaml:",inline"`

	// IdentityPath is fetched after login purely to name who signed in.
	IdentityPath string `yaml:"identityPath"`

	// TokenPath is the page where a human mints the credential by hand, for
	// the tools that have no login endpoint to drive. facile opens it rather
	// than telling the user to go and find it.
	TokenPath string `yaml:"tokenPath"`

	// Transport is how the credential is later presented: bearer or cookie.
	Transport  string `yaml:"transport"`
	CookieName string `yaml:"cookieName"`

	Env `yaml:",inline"`

	Store *Store `yaml:"store"`

	// Note explains a tool that cannot be logged into, so facile can say why
	// instead of pretending the command did something.
	Note string `yaml:"note"`
}

// Flows are the four handshakes a tool can declare. A tool may declare several
// — a device sign-in keeps its loopback flow, and both keep the tool's own
// password endpoint — and Kind says which one facile prefers.
type Flows struct {
	SSO        *SSOFlow        `yaml:"sso"`
	OIDCDevice *OIDCDeviceFlow `yaml:"oidcDevice"`
	Password   *PasswordFlow   `yaml:"password"`
	Device     *DeviceFlow     `yaml:"device"`
}

// Env is the pair of environment variables that override what facile stored.
// EnvToken matters at logout: a variable still set keeps working, and the user
// would otherwise think the logout failed.
type Env struct {
	EnvToken string `yaml:"envToken"`
	EnvURL   string `yaml:"envUrl"`
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

// OIDCDeviceFlow is RFC 8628 run against the suite's identity provider rather
// than against the tool. It exists because the loopback flow assumes the
// browser is on the machine that started the login: when it is not, the
// provider redirects the wrong machine's browser to 127.0.0.1 and the login
// hangs until the code expires. Here nothing is redirected anywhere — the user
// carries a short code to whatever device has the browser.
//
// A tool declaring this keeps its SSO block. The loopback flow stays the
// same-machine path and is what runs when the provider does not advertise the
// grant, so a catalog entry can name the device flow before the provider
// serves it.
type OIDCDeviceFlow struct {
	// Issuer is the OIDC issuer. Endpoints come from its discovery document,
	// never from paths assembled here: on this provider a per-application
	// discovery path answers 200 with the single-page app's HTML, which is a
	// false positive that has already misled two investigations.
	Issuer string `yaml:"issuer"`

	// ClientID names a public client. A CLI ships on the user's machine and
	// cannot keep a secret, so there is none here and none is sent.
	ClientID string `yaml:"clientId"`

	// Scopes are space separated, as OAuth spells them. profile and email are
	// what the app needs to name the account behind the token.
	Scopes string `yaml:"scopes"`

	// ExchangePath trades the provider's access token for the tool's own
	// credential, because what the tool reads is its own session and not the
	// provider's — the same reason the loopback flow exchanges a code rather
	// than storing it. The prefix mirrors this tool's SSO exchange path: an
	// app serving its API under /api serves both under /api.
	ExchangePath string `yaml:"exchangePath"`
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
func (t Tool) EnvToken() string {
	if t.Auth == nil {
		return ""
	}
	return t.Auth.EnvToken
}

// Federates reports whether this tool signs in through the shared identity
// provider. Those flows run first: the first of them authenticates and the
// rest complete without the user touching anything, which is only worth
// anything if they are not queued behind a prompt.
func (t Tool) Federates() bool {
	kind := t.AuthKind()
	return kind == "sso" || kind == "oidc-device"
}

// AuthKind is the flow this tool needs, or "none".
func (t Tool) AuthKind() string {
	if t.Auth == nil || t.Auth.Kind == "" {
		return "none"
	}
	return t.Auth.Kind
}

// TokenPage is where a human goes to mint the credential themselves. Only the
// tools with no login endpoint have one, and opening it beats printing a
// sentence that sends the user hunting through a dashboard for the right page.
func (t Tool) TokenPage(serverURL string) string {
	if t.Auth == nil || t.Auth.TokenPath == "" || serverURL == "" {
		return ""
	}
	return strings.TrimSuffix(serverURL, "/") + t.Auth.TokenPath
}
