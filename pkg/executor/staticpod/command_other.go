//go:build !linux
// +build !linux

package staticpod

import "os/exec"

func addDeathSig(_ *exec.Cmd) {
	// not supported in this OS
}
