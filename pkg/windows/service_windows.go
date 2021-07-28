// +build windows

package windows

import (
	"os"
	"time"

	"github.com/Freman/eventloghook"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

type service struct{}

var Service = &service{}

func (h *service) Execute(_ []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	statuses <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range requests {
		switch c.Cmd {
		case svc.Interrogate:
			statuses <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			statuses <- svc.Status{State: svc.StopPending}
			logrus.Info("Windows Service is shutting down in 5s")
			time.Sleep(5 * time.Second)
			return false, 0
		}
	}
	return false, 0
}

func StartService() error {
	if ok, err := svc.IsWindowsService(); err != nil || !ok {
		return err
	}

	elog, err := eventlog.Open(version.Program)
	if err != nil {
		return err
	}
	logrus.AddHook(eventloghook.NewHook(elog))

	stop := make(chan struct{})
	go watchService(stop)
	go func() {
		defer close(stop)
		if err := svc.Run(version.Program, Service); err != nil {
			logrus.Fatalf("Windows Service error, exiting: %s", err)
		}
	}()

	return nil
}

func watchService(stop chan struct{}) {
	<-stop // pause for service to be stopped
	ok, err := svc.IsWindowsService()
	if err != nil {
		logrus.Warnf("Error trying to determine if running as a Windows Service: %s", err)
	}

	if ok {
		logrus.Infof("Windows Service is shutting down")
		os.Exit(0)
	}
}
