// Package version carries this binary's build identity.
//
// The values are stamped at link time with -ldflags -X, so a running process
// can always answer "which commit am I?". That question sounds bureaucratic
// until it decides an argument: a security report once listed five defects,
// four of which did not reproduce because the binary under test was months
// behind the repository. Every later finding inherits that uncertainty, so
// build identity is the zeroth fact of a bug report, not a nicety.
//
// The defaults below deliberately look like what they are -- an unstamped
// build -- instead of a plausible release number. A missing stamp that reads
// "dev" makes someone go and check; a missing stamp that reads "1.0.0" gets
// quoted in a ticket as fact.
package version

import "fmt"

var (
	// Version is the semantic version from the VERSION file, e.g. "1.0.0".
	Version = "0.0.0-dev"
	// Commit is the short git sha this binary was built from.
	Commit = "unknown"
	// BuildTime is the UTC build timestamp in RFC 3339 form.
	BuildTime = "unknown"
	// Dirty is "true" when the working tree had uncommitted changes at build
	// time. A dirty build is a build nobody can reproduce from a tag, which is
	// worth saying out loud rather than hiding.
	Dirty = "false"
)

// String renders the full identity on one line: what `-version` prints and what
// the boot log carries.
func String() string {
	s := fmt.Sprintf("%s (commit %s, built %s", Version, Commit, BuildTime)
	if Dirty == "true" {
		s += ", dirty"
	}
	return s + ")"
}

// Short is the version plus commit, for a log line that already says which
// service it belongs to.
func Short() string {
	if Commit == "" || Commit == "unknown" {
		return Version
	}
	return Version + "+" + Commit
}
