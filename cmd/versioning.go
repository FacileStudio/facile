package cmd

import (
	"regexp"
	"strconv"
	"strings"
)

var semver = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:[-+](.+))?$`)

func isSemver(v string) bool { return semver.MatchString(v) }

type release struct {
	num [3]int
	pre string
}

// parseRelease reads a plain semver. Anything else — a commit SHA from a source
// build, a two-part version, an empty string — fails, and every caller treats a
// failure as "no claim to make" rather than as a difference.
func parseRelease(v string) (release, bool) {
	m := semver.FindStringSubmatch(v)
	if m == nil {
		return release{}, false
	}
	var r release
	for i := range r.num {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return release{}, false
		}
		r.num[i] = n
	}
	r.pre = m[4]
	return r, true
}

// before reports whether r precedes other. A prerelease sorts below the release
// it leads to, so 0.9.0-rc1 is behind 0.9.0 while 0.9.0 is behind nothing.
func (r release) before(other release) bool {
	for i := range r.num {
		if r.num[i] != other.num[i] {
			return r.num[i] < other.num[i]
		}
	}
	if (r.pre == "") != (other.pre == "") {
		return r.pre != ""
	}
	return r.pre < other.pre
}

// outdated reports whether have is strictly older than latest, and answers no
// whenever the question cannot be asked.
//
// It must be an ordering, never an inequality. The cached tag can lag the
// binary — install a release minutes after it publishes and the day-old cache
// still names the previous one — and comparing for difference renders that as
// `0.9.0 → 0.8.0`, an arrow pointing backwards at a downgrade.
//
// The unanswerable cases are equally deliberate. A source build reports a commit
// SHA, and a SHA cannot be ordered against a tag; `update` draws the opposite
// conclusion from that same unknown on purpose, because there an unresolved
// comparison costs a download while here it costs a false statement.
func outdated(have, latest string) bool {
	mine, ok := parseRelease(have)
	if !ok {
		return false
	}
	newest, ok := parseRelease(latest)
	if !ok {
		return false
	}
	return mine.before(newest)
}

// versionOf returns the version part of a line like "{tool} {version}" or
// "{tool} version {version}". It returns everything after the first space,
// unless the second word is "version", in which case it returns the third word.
func versionOf(line string) string {
	parts := strings.Fields(line)
	if len(parts) >= 3 && parts[1] == "version" {
		return parts[2]
	}
	if _, ver, found := strings.Cut(line, " "); found {
		return ver
	}
	return line
}
