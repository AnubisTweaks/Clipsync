package action

import (
	"github.com/lxn/walk"
)

func NewExitAction() (*walk.Action, error) {
	action := walk.NewAction()
	if err := action.SetText("Exit"); err != nil {
		return nil, err
	}

	action.Triggered().Attach(exitHandler)
	return action, nil
}

func exitHandler() {
	walk.App().Exit(0)
}
