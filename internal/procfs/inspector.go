// Package procfs provides small Linux /proc process-tree helpers.
package procfs

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Inspector validates process ancestry.
type Inspector interface {
	IsAncestor(ancestorPID, childPID int) (bool, error)
}

// LinuxInspector reads /proc/<pid>/stat.
type LinuxInspector struct{}

// IsAncestor reports whether ancestorPID appears in childPID's parent chain.
func (LinuxInspector) IsAncestor(ancestorPID, childPID int) (bool, error) {
	if ancestorPID <= 0 || childPID <= 0 {
		return false, fmt.Errorf("invalid pid ancestry check: ancestor=%d child=%d", ancestorPID, childPID)
	}
	seen := make(map[int]bool)
	pid := childPID
	for pid > 1 {
		if pid == ancestorPID {
			return true, nil
		}
		if seen[pid] {
			return false, fmt.Errorf("cycle in process parent chain at pid %d", pid)
		}
		seen[pid] = true
		ppid, err := parentPID(pid)
		if err != nil {
			return false, err
		}
		pid = ppid
	}
	return false, nil
}

func parentPID(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	return parentPIDFromStat(pid, string(data))
}

func parentPIDFromStat(pid int, stat string) (int, error) {
	end := strings.LastIndex(stat, ")")
	if end == -1 || end+2 >= len(stat) {
		return 0, fmt.Errorf("malformed proc stat for pid %d", pid)
	}
	fields := strings.Fields(stat[end+2:])
	if len(fields) < 2 {
		return 0, fmt.Errorf("malformed proc stat fields for pid %d", pid)
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("invalid ppid for pid %d: %w", pid, err)
	}
	return ppid, nil
}
