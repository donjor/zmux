package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// confirm prints prompt and reads a y/N answer from stdin. Anything but an
// explicit yes (including EOF, e.g. non-interactive callers) declines.
func confirm(prompt string) bool {
	fmt.Printf("%s (y/N) ", prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
