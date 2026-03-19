package dashboard

// SwitchTabIntent requests the app switch to a specific tab.
type SwitchTabIntent struct {
	Tab TabID
}

func (SwitchTabIntent) AppIntent() {}

// SetStatusIntent requests the app display a status flash message.
type SetStatusIntent struct {
	Text    string
	IsError bool
}

func (SetStatusIntent) AppIntent() {}

// QuitIntent requests the app terminate.
type QuitIntent struct {
	// Action and Chosen carry the result for the caller.
	Action string
	Chosen string
}

func (QuitIntent) AppIntent() {}
