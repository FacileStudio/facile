package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrNoFacileToml is returned when neither facile.toml nor a [facile]
// block in mise.toml can be found in a repository.
var ErrNoFacileToml = fmt.Errorf("no facile.toml or [facile] block in mise.toml")

// parseFacileToml reads a facile.toml byte slice and returns the
// parsed configuration. The file is produced by repos that opt into
// the extension plan.
func parseFacileToml(raw []byte) (*FacileToml, error) {
	var ft FacileToml
	if _, err := toml.Decode(string(raw), &ft); err != nil {
		return nil, fmt.Errorf("parsing facile.toml: %w", err)
	}
	if ft.Name == "" {
		return nil, fmt.Errorf("facile.toml: name is required")
	}
	if ft.Repo == "" {
		return nil, fmt.Errorf("facile.toml: repo is required")
	}
	if ft.VersionPattern != "" && !validateVersionPattern(ft.VersionPattern) {
		return nil, fmt.Errorf("facile.toml: invalid versionPattern: %s", ft.VersionPattern)
	}
	return &ft, nil
}

// parseMiseTomlFacade looks for a [facile] block in a mise.toml
// byte slice. mise.toml uses TOML, so we decode the whole file and
// extract the section. Repos that do not ship a facile.toml fall
// back here; a malformed or missing [facile] block is silently skipped.
func parseMiseTomlFacade(raw []byte) (*FacileToml, error) {
	var generic struct {
		Facile *FacileToml `toml:"facile"`
	}
	if _, err := toml.Decode(string(raw), &generic); err != nil {
		return nil, fmt.Errorf("parsing mise.toml: %w", err)
	}
	if generic.Facile == nil {
		return nil, ErrNoFacileToml
	}
	if generic.Facile.Name == "" {
		return nil, fmt.Errorf("facile.toml: name is required")
	}
	if generic.Facile.Repo == "" {
		return nil, fmt.Errorf("facile.toml: repo is required")
	}
	if generic.Facile.VersionPattern != "" && !validateVersionPattern(generic.Facile.VersionPattern) {
		return nil, fmt.Errorf("facile.toml: invalid versionPattern: %s", generic.Facile.VersionPattern)
	}
	return generic.Facile, nil
}

// ToTool converts a FacileToml into a Tool with sensible defaults.
// Branch defaults to "main"; Build defaults to "go"; Bin defaults
// to the tool name; SrcSubdir defaults to ".". Returns an error
// when VersionPattern is invalid.
func (ft *FacileToml) ToTool() (Tool, error) {
	if ft.VersionPattern != "" && !validateVersionPattern(ft.VersionPattern) {
		return Tool{}, fmt.Errorf("facile.toml: invalid versionPattern: %s", ft.VersionPattern)
	}
	return rawTool{
		name: ft.Name, summary: ft.Summary, repo: ft.Repo, branch: ft.Branch,
		build: ft.Build, bin: ft.Bin, srcSubdir: ft.SrcSubdir, asset: ft.Asset,
		skill: ft.Skill, goVersionVar: ft.GoVersionVar, requires: ft.Requires,
		versionCmd: ft.VersionCmd, versionPattern: ft.VersionPattern,
	}.toTool(), nil
}

// facileTomlRaw is a minimal TOML decoder used only to probe for the
// existence of a facile.toml file without full validation — a TOML
// file that parses but lacks a name is treated as not a facile.toml,
// falling through to mise.toml probing.
func facileTomlProbe(raw []byte) bool {
	var ft struct {
		Name string `toml:"name"`
	}
	_, err := toml.Decode(string(raw), &ft)
	return err == nil && ft.Name != ""
}

// miseTomlFacadeProbe checks whether a mise.toml carries a [facile] block
// with at least a name. Used by the discovery pipeline to decide which
// file to present to the user in doctor.
func miseTomlFacadeProbe(raw []byte) bool {
	var generic struct {
		Facile *struct {
			Name string `toml:"name"`
		} `toml:"facile"`
	}
	_, err := toml.Decode(string(raw), &generic)
	return err == nil && generic.Facile != nil && generic.Facile.Name != ""
}

// validateVersionPattern compiles a versionPattern regex and returns
// true if it is valid. A bad pattern would make update think every
// tool is current, so we reject it at load time.
func validateVersionPattern(pattern string) bool {
	if pattern == "" {
		return true
	}
	_, err := regexp.Compile(pattern)
	return err == nil
}

// MatchVersion applies a versionPattern regex to the output of
// versionCmd (or the --version line) and returns the captured
// version string. If the pattern has no capture groups the whole
// match is returned. An invalid or non-matching pattern returns
// the empty string, which makes the caller fall back to the default
// heuristic.
func MatchVersion(output, pattern string) string {
	if pattern == "" || output == "" {
		return ""
	}
	re := regexpCompile(pattern)
	if re == nil {
		return ""
	}
	m := re.FindStringSubmatch(strings.TrimSpace(output))
	if m == nil {
		return ""
	}
	if len(m) >= 2 {
		return m[1]
	}
	return m[0]
}

// regexpCompile wraps regexp.Compile and returns nil on failure
// rather than panicking. Bad patterns are a load-time validation
// concern, not a runtime one.
func regexpCompile(pattern string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}
