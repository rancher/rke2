package shared

import ()

type Service struct {
	SCName      string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      uint32 `json:"status"`
	StatusText  string `json:"statusText"`
	ServiceType uint32 `json:"serviceType"`
	IsRunning   bool   `json:"isRunning"`
	AcceptStop  bool   `json:"acceptStop"`
	RunningPid  uint32 `json:"pid"`
}
