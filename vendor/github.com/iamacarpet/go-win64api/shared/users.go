package shared

import (
	"time"
)

type LocalUser struct {
	Username             string        `json:"username"`
	FullName             string        `json:"fullName"`
	IsEnabled            bool          `json:"isEnabled"`
	IsLocked             bool          `json:"isLocked"`
	IsAdmin              bool          `json:"isAdmin"`
	PasswordNeverExpires bool          `json:"passwordNeverExpires"`
	NoChangePassword     bool          `json:"noChangePassword"`
	PasswordAge          time.Duration `json:"passwordAge"`
	LastLogon            time.Time     `json:"lastLogon"`
	BadPasswordCount     uint32        `json:"badPasswordCount"`
	NumberOfLogons       uint32        `json:"numberOfLogons"`
}
