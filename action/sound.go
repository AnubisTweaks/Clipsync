package action

import (
	"github.com/lxn/walk"
)

func NewSoundAction(getSoundEnabled func() bool, setSoundEnabled func(bool)) (*walk.Action, error) {
	action := walk.NewAction()
	if err := action.SetText("Play Sound"); err != nil {
		return nil, err
	}

	if err := action.SetCheckable(true); err != nil {
		return nil, err
	}

	// Set initial state
	action.SetChecked(getSoundEnabled())

	action.Triggered().Attach(func() {
		// Toggle sound state
		newState := action.Checked()
		setSoundEnabled(newState)
	})

	return action, nil
}
