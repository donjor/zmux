package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GetBatchProcessStats fetches stats for multiple PIDs using a single ps call.
// Returns a map of pid → ProcessStats. One subprocess total, not one per PID.
func GetBatchProcessStats(pids []int) map[int]ProcessStats {
	result := make(map[int]ProcessStats, len(pids))
	if len(pids) == 0 {
		return result
	}

	// Single ps call: get ALL processes with ppid, pid, cpu, rss, etime.
	// Then filter/aggregate in Go. This is one fork instead of N.
	out, err := exec.Command("ps", "-e", "-o", "ppid,pid,%cpu,rss,etime",
		"--no-headers").Output()
	if err != nil {
		return result
	}

	// Build a set of PIDs we care about for fast lookup.
	pidSet := make(map[int]bool, len(pids))
	for _, p := range pids {
		pidSet[p] = true
	}

	// Parse all processes and aggregate by parent PID.
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}

		ppid, _ := strconv.Atoi(fields[0])
		pid, _ := strconv.Atoi(fields[1])

		// Include if this process IS one of our targets or is a CHILD of one.
		targetPID := 0
		if pidSet[pid] {
			targetPID = pid
		} else if pidSet[ppid] {
			targetPID = ppid
		}
		if targetPID == 0 {
			continue
		}

		cpu, _ := strconv.ParseFloat(fields[2], 64)
		rss, _ := strconv.ParseInt(fields[3], 10, 64)
		etime := fields[4]

		stats := result[targetPID]
		stats.CPU += cpu
		stats.MemMB += float64(rss) / 1024.0
		if stats.Uptime == "" || len(etime) > len(stats.Uptime) {
			stats.Uptime = formatUptime(etime)
		}
		result[targetPID] = stats
	}

	return result
}

// GetProcessStats returns aggregated CPU/memory stats for a process tree
// rooted at the given PID. It shells out to ps to gather child process stats.
// Returns zero values if ps fails or the PID is invalid.
func GetProcessStats(pid int) ProcessStats {
	if pid <= 0 {
		return ProcessStats{}
	}

	// Get child processes' CPU, RSS, and elapsed time.
	out, err := exec.Command("ps", "--ppid", strconv.Itoa(pid),
		"-o", "%cpu,rss,etime", "--no-headers").Output()
	if err != nil {
		// No children or ps failed — try the process itself.
		out, err = exec.Command("ps", "-p", strconv.Itoa(pid),
			"-o", "%cpu,rss,etime", "--no-headers").Output()
		if err != nil {
			return ProcessStats{}
		}
	}

	return parseProcessStats(string(out))
}

// parseProcessStats aggregates ps output lines into a single ProcessStats.
func parseProcessStats(output string) ProcessStats {
	output = strings.TrimSpace(output)
	if output == "" {
		return ProcessStats{}
	}

	var totalCPU float64
	var totalRSS int64
	var longestUptime string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}

		cpu, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		totalCPU += cpu

		rss, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		totalRSS += rss

		// Keep the longest uptime string (last field).
		etime := fields[len(fields)-1]
		if longestUptime == "" || len(etime) > len(longestUptime) {
			longestUptime = etime
		}
	}

	memMB := float64(totalRSS) / 1024.0

	return ProcessStats{
		CPU:    totalCPU,
		MemMB:  memMB,
		Uptime: formatUptime(longestUptime),
	}
}

// formatUptime converts ps etime format to a human-friendly string.
// ps etime formats: "MM:SS", "HH:MM:SS", "D-HH:MM:SS"
func formatUptime(etime string) string {
	etime = strings.TrimSpace(etime)
	if etime == "" {
		return ""
	}

	// Handle day-separated format: "D-HH:MM:SS"
	days := 0
	if idx := strings.Index(etime, "-"); idx > 0 {
		d, err := strconv.Atoi(etime[:idx])
		if err == nil {
			days = d
		}
		etime = etime[idx+1:]
	}

	parts := strings.Split(etime, ":")
	hours, minutes := 0, 0

	switch len(parts) {
	case 3: // HH:MM:SS
		hours, _ = strconv.Atoi(parts[0])
		minutes, _ = strconv.Atoi(parts[1])
	case 2: // MM:SS
		minutes, _ = strconv.Atoi(parts[0])
	}

	totalMinutes := days*24*60 + hours*60 + minutes

	switch {
	case totalMinutes >= 24*60:
		return fmt.Sprintf("%dd %dh", totalMinutes/(24*60), (totalMinutes%(24*60))/60)
	case totalMinutes >= 60:
		return fmt.Sprintf("%dh %dm", totalMinutes/60, totalMinutes%60)
	case totalMinutes > 0:
		return fmt.Sprintf("%dm", totalMinutes)
	default:
		return "<1m"
	}
}
