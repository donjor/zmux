package procfs

import (
	"os"
	"testing"
)

func TestParentPIDFromStat(t *testing.T) {
	tests := []struct {
		name string
		stat string
		want int
	}{
		{name: "normal", stat: "123 (bash) S 42 1 1 0 0", want: 42},
		{name: "comm with spaces", stat: "123 (my shell) S 77 1 1 0 0", want: 77},
		{name: "comm with parenthesis", stat: "123 (odd)name) S 88 1 1 0 0", want: 88},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parentPIDFromStat(123, tt.stat)
			if err != nil {
				t.Fatalf("parentPIDFromStat returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("parentPIDFromStat = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParentPIDFromStatErrors(t *testing.T) {
	tests := []struct {
		name string
		stat string
	}{
		{name: "missing closing paren", stat: "123 (bash S 42 1 1"},
		{name: "too few fields", stat: "123 (bash) S"},
		{name: "invalid ppid", stat: "123 (bash) S nope 1 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parentPIDFromStat(123, tt.stat); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLinuxInspectorCurrentProcessAncestry(t *testing.T) {
	ppid := os.Getppid()
	pid := os.Getpid()
	if ppid <= 1 {
		t.Skip("current process parent is not stable enough for ancestry test")
	}
	ok, err := (LinuxInspector{}).IsAncestor(ppid, pid)
	if err != nil {
		t.Fatalf("IsAncestor returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected parent pid %d to be ancestor of pid %d", ppid, pid)
	}
}

func TestLinuxInspectorRejectsInvalidPID(t *testing.T) {
	if _, err := (LinuxInspector{}).IsAncestor(0, os.Getpid()); err == nil {
		t.Fatal("expected invalid ancestor pid error")
	}
	if _, err := (LinuxInspector{}).IsAncestor(os.Getppid(), 0); err == nil {
		t.Fatal("expected invalid child pid error")
	}
}
