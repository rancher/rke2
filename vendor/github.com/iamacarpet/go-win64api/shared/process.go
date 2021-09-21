package shared

import ()

type Process struct {
	Pid        int    `json:"pid"`
	Ppid       int    `json:"parentpid"`
	Executable string `json:"exeName"`
	Fullpath   string `json:"fullPath"`
	Username   string `json:"username"`
}
