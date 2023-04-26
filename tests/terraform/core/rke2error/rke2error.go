package rke2error

import (
	"fmt"
)

type Rke2Error struct {
	ErrorSource string
	Action      interface{}
	Message     string
	Err         error
}

func (r Rke2Error) Error() string {
	if r.Err != nil {
		return fmt.Sprintf("ErrorSource: %s, Action: %v, Message: %s, Err: %s",
			r.ErrorSource, r.Action, r.Message, r.Err)
	}

	return fmt.Sprintf("ErrorSource: %s, Action: %v, Message: %s",
		r.ErrorSource, r.Action, r.Message)
}

func NewRke2Error(errorSource string, action interface{}, message string, err error) error {
	return Rke2Error{
		ErrorSource: errorSource,
		Action:      action,
		Message:     message,
		Err:         err,
	}
}
