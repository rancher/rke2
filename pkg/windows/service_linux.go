//go:build !windows
// +build !windows

package windows

func StartService() (bool, error) {
	return false, nil
}

func MonitorProcessExit() {}
