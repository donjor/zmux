package cli

import (
	"regexp"
	"strings"

	"github.com/donjor/zmux/internal/waitfor"
)

// readinessEchoesCommand reports whether an output-readiness regex would be
// satisfied by the launch command's own terminal echo rather than by real
// startup output. Delivered input is redisplayed at the shell prompt, so a
// pattern that also matches the command proves nothing about the process
// actually starting. Callers reject such a pattern up front.
//
// An empty readiness or command is never an echo match. A malformed pattern
// surfaces as an error so callers report it as an invalid regex, not a false
// negative.
func readinessEchoesCommand(readiness, command string) (bool, error) {
	if strings.TrimSpace(readiness) == "" || strings.TrimSpace(command) == "" {
		return false, nil
	}
	condition, err := waitfor.ParseCondition("output:" + readiness)
	if err != nil {
		return false, err
	}
	return regexp.MatchString("(?i)"+condition.Value, command)
}
