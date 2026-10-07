package manifest

import "fmt"

// VersionDetection holds the command and regex pattern used to extract
// a semantic version from a tool's output.
type VersionDetection struct {
	Cmd     string `yaml:"versionCmd"`
	Pattern string `yaml:"versionPattern"`
}

// Tool is one installable CLI. The field names mirror the config block
// of the per-repo install.sh so a divergence between the two is easy to spot.
type Tool struct {
	Name         string   `yaml:"name"`
	Summary      string   `yaml:"summary"`
	Repo         string   `yaml:"repo"`
	Branch       string   `yaml:"branch"`
	Bin          string   `yaml:"bin"`
	Build        string   `yaml:"build"`
	SrcSubdir    string   `yaml:"srcSubdir"`
	Asset        string   `yaml:"asset"`
	Skill        string   `yaml:"skill"`
	GoVersionVar string   `yaml:"goVersionVar"`
	Requires     []string `yaml:"requires"`
	Auth         *Auth    `yaml:"auth"`

	VersionDetection `yaml:",inline"`
}

// Manifest is the whole catalog. MCP names the tools that register their MCP
// server in agent harnesses on install, through each one's own install
// subcommand; it is a list off Tool because Tool sits at the 12-field cap.
type Manifest struct {
	Version int      `yaml:"version"`
	Tools   []Tool   `yaml:"tools"`
	MCP     []string `yaml:"mcp"`

	// LoadErrors holds non-fatal errors from merging user sources:
	// unreachable list URLs, unparseable repo configs, and broken
	// SingleSource entries.
	LoadErrors []string `yaml:"-"`
}

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

// SourceConfig holds the user-defined sources configuration. It is read from
// ~/facile.yml or ~/.config/facile/sources.yml (first found wins).
type SourceConfig struct {
	Single []SingleSource `yaml:"single"`
	Lists  []ListSource   `yaml:"lists"`
}

// SingleSource is an explicit Tool entry in a user sources file.
type SingleSource struct {
	Name         string   `yaml:"name"`
	Summary      string   `yaml:"summary"`
	Repo         string   `yaml:"repo"`
	Branch       string   `yaml:"branch"`
	Bin          string   `yaml:"bin"`
	Build        string   `yaml:"build"`
	SrcSubdir    string   `yaml:"srcSubdir"`
	Asset        string   `yaml:"asset"`
	Skill        string   `yaml:"skill"`
	GoVersionVar string   `yaml:"goVersionVar"`
	Requires     []string `yaml:"requires"`

	VersionDetection `yaml:",inline"`
}

// ToTool converts a SingleSource into a Tool with sensible defaults.
// Branch defaults to "main"; Build defaults to "go"; Bin defaults
// to the tool name; SrcSubdir defaults to ".". Returns an error
// when Name is empty or VersionPattern is invalid.
func (s *SingleSource) ToTool() (Tool, error) {
	if s.Name == "" {
		return Tool{}, fmt.Errorf("single source entry missing name")
	}
	branch, build, bin, srcSubdir := s.Branch, s.Build, s.Bin, s.SrcSubdir
	if branch == "" {
		branch = "main"
	}
	if build == "" {
		build = "go"
	}
	if bin == "" {
		bin = s.Name
	}
	if srcSubdir == "" {
		srcSubdir = "."
	}
	if s.Pattern != "" && !validateVersionPattern(s.Pattern) {
		return Tool{}, fmt.Errorf("single source %q: invalid versionPattern: %s", s.Name, s.Pattern)
	}
	return Tool{
		Name:         s.Name,
		Summary:      s.Summary,
		Repo:         s.Repo,
		Branch:       branch,
		Bin:          bin,
		Build:        build,
		SrcSubdir:    srcSubdir,
		Asset:        s.Asset,
		Skill:        s.Skill,
		GoVersionVar: s.GoVersionVar,
		Requires:     s.Requires,
		VersionDetection: VersionDetection{
			Cmd:     s.Cmd,
			Pattern: s.Pattern,
		},
	}, nil
}

// ListSource is a URL that returns a list of repo identifiers, either one
// per line or as a PEP 503 simple index (HTML with <a href="/owner/repo/">).
type ListSource struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// FacileToml is the per-repo configuration file that external tools can ship
// to describe how facile should install them. It mirrors Tool fields but is
// TOML, not YAML.
type FacileToml struct {
	Name           string   `toml:"name"`
	Summary        string   `toml:"summary"`
	Repo           string   `toml:"repo"`
	Branch         string   `toml:"branch"`
	Bin            string   `toml:"bin"`
	Build          string   `toml:"build"`
	SrcSubdir      string   `toml:"srcSubdir"`
	Asset          string   `toml:"asset"`
	Skill          string   `toml:"skill"`
	GoVersionVar   string   `toml:"goVersionVar"`
	Requires       []string `toml:"requires"`
	VersionCmd     string   `toml:"versionCmd"`
	VersionPattern string   `toml:"versionPattern"`
}