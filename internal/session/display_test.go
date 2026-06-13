package session

import "testing"

func TestSessionDisplayNamesPreferManagedMetadata(t *testing.T) {
	s := SessionInfo{
		Name:      "zws_skills__skills-peer",
		Workspace: "skills",
		Label:     "skills-peer",
	}

	if got := LocalDisplayName(s); got != "skills-peer" {
		t.Fatalf("LocalDisplayName = %q", got)
	}
	if got := QualifiedDisplayName(s); got != "skills/skills-peer" {
		t.Fatalf("QualifiedDisplayName = %q", got)
	}
}

func TestSessionDisplayNamesFallBackToRawName(t *testing.T) {
	s := SessionInfo{Name: "legacy"}

	if got := LocalDisplayName(s); got != "legacy" {
		t.Fatalf("LocalDisplayName = %q", got)
	}
	if got := QualifiedDisplayName(s); got != "legacy" {
		t.Fatalf("QualifiedDisplayName = %q", got)
	}
}
