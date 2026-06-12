package session

import (
	"regexp"
	"strings"
)

const managedRawPrefix = "zws_"

var (
	// managedGroupSuffix matches the suffix used by grouped copies of managed
	// workspace sessions. This must stay in sync with nextGroupName in actions.go.
	managedGroupSuffix = regexp.MustCompile(`__clone_[b-z]$`)
	// legacyGroupSuffix matches the legacy -b through -z suffix used by grouped
	// copies of unmanaged sessions.
	legacyGroupSuffix = regexp.MustCompile(`-[b-z]$`)
)

// RootName strips grouped-session clone suffixes, returning the
// root session name. If the name has no group suffix it is returned as-is.
func RootName(name string) string {
	if strings.HasPrefix(name, managedRawPrefix) {
		return managedGroupSuffix.ReplaceAllString(name, "")
	}
	root := managedGroupSuffix.ReplaceAllString(name, "")
	return legacyGroupSuffix.ReplaceAllString(root, "")
}
