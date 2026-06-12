package workspace

import "fmt"

type sessionOptionSetter interface {
	SetSessionOption(target, key, value string) error
}

// StampSessionMetadata writes zmux identity metadata onto a tmux session.
func StampSessionMetadata(runner sessionOptionSetter, workspaceName string, rec WorkspaceSession) error {
	if err := runner.SetSessionOption(rec.TmuxName, OptionManaged, "1"); err != nil {
		return fmt.Errorf("set managed option: %w", err)
	}
	if err := runner.SetSessionOption(rec.TmuxName, OptionWorkspace, workspaceName); err != nil {
		return fmt.Errorf("set workspace option: %w", err)
	}
	if err := runner.SetSessionOption(rec.TmuxName, OptionSessionLabel, rec.Label); err != nil {
		return fmt.Errorf("set session label option: %w", err)
	}
	if err := runner.SetSessionOption(rec.TmuxName, OptionSessionID, rec.ID); err != nil {
		return fmt.Errorf("set session id option: %w", err)
	}
	return nil
}
