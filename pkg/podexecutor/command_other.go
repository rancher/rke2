// +build !linux

package podexecutor

import "os/exec"

func addDeathSig(_ *exec.Cmd) {
	// not supported in this OS
}
