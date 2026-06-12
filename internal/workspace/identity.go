package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	ManagedSessionPrefix = "zws_"

	OptionManaged      = "@zmux_managed"
	OptionWorkspace    = "@zmux_workspace"
	OptionSessionLabel = "@zmux_session_label"
	OptionSessionID    = "@zmux_session_id"
)

// ValidateWorkspaceName checks if a workspace name is valid.
// Workspaces are globally unique and form the left side of workspace/session.
func ValidateWorkspaceName(name string) error {
	return validateIdentityPart("workspace name", name)
}

// ValidateSessionLabel checks if a workspace-local session label is valid.
func ValidateSessionLabel(label string) error {
	if err := validateIdentityPart("session label", label); err != nil {
		return err
	}
	if label[0] >= '0' && label[0] <= '9' {
		return fmt.Errorf("session label cannot start with a number (reserved for quick-select)")
	}
	return nil
}

func validateIdentityPart(kind, value string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", kind)
	}
	if value == "__external__" || value == "temporary" {
		return fmt.Errorf("%s %q is reserved", kind, value)
	}
	for _, r := range value {
		switch {
		case r == '/' || r == ':':
			return fmt.Errorf("%s cannot contain %q: %q", kind, r, value)
		case unicode.IsSpace(r) || unicode.IsControl(r):
			return fmt.Errorf("%s cannot contain whitespace or control characters: %q", kind, value)
		}
	}
	return nil
}

// NewSessionRecord returns the canonical identity record for workspace/label.
func NewSessionRecord(workspace, label string) (WorkspaceSession, error) {
	if err := ValidateWorkspaceName(workspace); err != nil {
		return WorkspaceSession{}, err
	}
	if err := ValidateSessionLabel(label); err != nil {
		return WorkspaceSession{}, err
	}
	now := time.Now()
	id := StableSessionID(workspace, label)
	return WorkspaceSession{
		ID:        id,
		Label:     label,
		TmuxName:  RawSessionName(workspace, label),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// StableSessionID is deterministic so state can be reconstructed from
// workspace/session metadata when old v2 state is migrated or repaired.
func StableSessionID(workspace, label string) string {
	sum := sha256.Sum256([]byte(workspace + "\x00" + label))
	return "s_" + hex.EncodeToString(sum[:])[:12]
}

// RawSessionName returns the generated tmux session name for workspace/label.
func RawSessionName(workspace, label string) string {
	return ManagedSessionPrefix + encodePart(workspace) + "__" + encodePart(label)
}

// IsManagedTmuxName reports whether name looks like a generated zmux session.
func IsManagedTmuxName(name string) bool {
	return strings.HasPrefix(name, ManagedSessionPrefix)
}

// ParseRawSessionName decodes a generated raw tmux session name. Tmux options
// remain authoritative; this is only a recovery/debug fallback.
func ParseRawSessionName(name string) (workspace, label string, ok bool) {
	if !IsManagedTmuxName(name) {
		return "", "", false
	}
	body := strings.TrimPrefix(name, ManagedSessionPrefix)
	parts := strings.SplitN(body, "__", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	ws, err := decodePart(parts[0])
	if err != nil {
		return "", "", false
	}
	lbl, err := decodePart(parts[1])
	if err != nil {
		return "", "", false
	}
	return ws, lbl, true
}

func encodePart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if isRawSafe(r) {
			b.WriteRune(r)
			continue
		}
		buf := []byte(string(r))
		for _, by := range buf {
			fmt.Fprintf(&b, "_%02X", by)
		}
	}
	return b.String()
}

func isRawSafe(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-' ||
		r == '.'
}

func decodePart(value string) (string, error) {
	var out []byte
	for i := 0; i < len(value); i++ {
		if value[i] != '_' {
			out = append(out, value[i])
			continue
		}
		if i+2 >= len(value) {
			return "", fmt.Errorf("truncated escape")
		}
		by, err := hex.DecodeString(value[i+1 : i+3])
		if err != nil {
			return "", err
		}
		out = append(out, by[0])
		i += 2
	}
	return string(out), nil
}
