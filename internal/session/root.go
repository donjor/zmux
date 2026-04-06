package session

import "regexp"

// groupSuffix matches the -b through -z suffix used by grouped sessions.
// This must stay in sync with nextGroupName in actions.go.
var groupSuffix = regexp.MustCompile(`-[b-z]$`)

// RootName strips the -b through -z grouped session suffix, returning the
// root session name. If the name has no group suffix it is returned as-is.
func RootName(name string) string {
	return groupSuffix.ReplaceAllString(name, "")
}
