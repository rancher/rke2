//go:build windows
// +build windows

package cmds

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
	syswin "golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	defaultServiceDescription = "Rancher Kubernetes Engine v2 (agent) see docs https://github.com/rancher/rke2#readme"
)

var serviceSubcommand = &cli.Command{
	Name:   "service",
	Usage:  "Manage RKE2 as a Windows Service",
	Action: serviceAction,
	Flags: []cli.Flag{
		cmds.ConfigFlag,
		&cli.BoolFlag{
			Name:  "add",
			Usage: "add RKE2 as a Windows Service",
		},
		&cli.BoolFlag{
			Name:  "delete",
			Usage: "stop and delete RKE2 as a Windows Service",
		},
		&cli.StringFlag{
			Name:  "service-name",
			Usage: "name for the RKE2 service",
			Value: version.Program,
		},
	},
}

func serviceAction(ctx *cli.Context) error {
	add := ctx.Bool("add")
	del := ctx.Bool("delete")
	if (!add && !del) || (add && del) {
		return errors.New("service subcommand requires one of --add or --delete")
	}

	if del {
		return deleteWindowsService(ctx.String("service-name"))
	}

	return addWindowService(ctx.String("service-name"), ctx.String("config"))
}

func getServicePath() (string, error) {
	p, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

// inspiration https://github.com/containerd/containerd/blob/main/cmd/containerd/command/service_windows.go#L127
func addWindowService(serviceName, config string) error {
	p, err := getServicePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.CreateService(serviceName, p, mgr.Config{
		ServiceType:  syswin.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  version.Program,
		Description:  defaultServiceDescription,
	}, "agent", "--config", config)
	if err != nil {
		return err
	}
	defer s.Close()

	const (
		scActionRestart             = 1
		serviceConfigFailureActions = 2
	)

	type scAction struct {
		Type  uint32
		Delay uint32
	}

	t := []scAction{
		{Type: scActionRestart, Delay: uint32(30 * time.Second / time.Millisecond)}, // on failure restart every 30s
	}

	type serviceFailureActions struct {
		ResetPeriod  uint32
		RebootMsg    *uint16
		Command      *uint16
		ActionsCount uint32
		Actions      uintptr
	}

	lpInfo := serviceFailureActions{ResetPeriod: uint32(30), ActionsCount: uint32(1), Actions: uintptr(unsafe.Pointer(&t[0]))}
	return syswin.ChangeServiceConfig2(s.Handle, serviceConfigFailureActions, (*byte)(unsafe.Pointer(&lpInfo)))
}

func deleteWindowsService(serviceName string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Delete()
}
