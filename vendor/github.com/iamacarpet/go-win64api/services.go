// +build windows,amd64

package winapi

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	so "github.com/iamacarpet/go-win64api/shared"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Windows API functions
var (
	svcEnumServicesStatusEx = modAdvapi32.NewProc("EnumServicesStatusExW")
)

const (
	SVC_SC_ENUM_PROCESS_INFO = 0
	SVC_SERVICE_WIN32        = 0x00000030
	SVC_SERVICE_STATE_ALL    = 0x00000003
	SVC_SERVICE_ACCEPT_STOP  = 0x00000001
)

type ENUM_SERVICE_STATUS_PROCESS struct {
	lpServiceName        *uint16
	lpDisplayName        *uint16
	ServiceStatusProcess SERVICE_STATUS_PROCESS
}

type SERVICE_STATUS_PROCESS struct {
	dwServiceType             uint32
	dwCurrentState            uint32
	dwControlsAccepted        uint32
	dwWin32ExitCode           uint32
	dwServiceSpecificExitCode uint32
	dwCheckPoint              uint32
	dwWaitHint                uint32
	dwProcessId               uint32
	dwServiceFlags            uint32
}

func GetServices() ([]so.Service, error) {
	hPointer, err := syscall.UTF16PtrFromString("")
	handle, err := windows.OpenSCManager(hPointer, nil, windows.SC_MANAGER_ENUMERATE_SERVICE|windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, fmt.Errorf("Error opening SCManager connection. %s", err.Error())
	}
	defer windows.CloseServiceHandle(handle)

	var (
		bytesReq     uint32
		numReturned  uint32
		resumeHandle uint32
		retData      []so.Service = make([]so.Service, 0)
	)

	_, _, _ = svcEnumServicesStatusEx.Call(
		uintptr(handle),
		uintptr(uint32(SVC_SC_ENUM_PROCESS_INFO)),
		uintptr(uint32(SVC_SERVICE_WIN32)),
		uintptr(uint32(SVC_SERVICE_STATE_ALL)),
		uintptr(0),
		0,
		uintptr(unsafe.Pointer(&bytesReq)),
		uintptr(unsafe.Pointer(&numReturned)),
		uintptr(unsafe.Pointer(&resumeHandle)),
		uintptr(0),
	)

	if bytesReq > 0 {
		var buf []byte = make([]byte, bytesReq)

		ret, _, _ := svcEnumServicesStatusEx.Call(
			uintptr(handle),
			uintptr(uint32(SVC_SC_ENUM_PROCESS_INFO)),
			uintptr(uint32(SVC_SERVICE_WIN32)),
			uintptr(uint32(SVC_SERVICE_STATE_ALL)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(bytesReq),
			uintptr(unsafe.Pointer(&bytesReq)),
			uintptr(unsafe.Pointer(&numReturned)),
			uintptr(unsafe.Pointer(&resumeHandle)),
			uintptr(0),
		)

		if ret > 0 {
			var sizeTest ENUM_SERVICE_STATUS_PROCESS
			iter := uintptr(unsafe.Pointer(&buf[0]))

			for i := uint32(0); i < numReturned; i++ {
				var data *ENUM_SERVICE_STATUS_PROCESS = (*ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(iter))

				rData := so.Service{
					SCName:      syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(data.lpServiceName))[:]),
					DisplayName: syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(data.lpDisplayName))[:]),
					Status:      data.ServiceStatusProcess.dwCurrentState,
					ServiceType: data.ServiceStatusProcess.dwServiceType,
					IsRunning:   (data.ServiceStatusProcess.dwCurrentState != windows.SERVICE_STOPPED),
					AcceptStop:  ((SVC_SERVICE_ACCEPT_STOP & data.ServiceStatusProcess.dwControlsAccepted) == SVC_SERVICE_ACCEPT_STOP),
					RunningPid:  data.ServiceStatusProcess.dwProcessId,
				}
				st := svc.State(rData.Status)
				if st == svc.Stopped {
					rData.StatusText = "Stopped"
				} else if st == svc.StartPending {
					rData.StatusText = "Start Pending"
				} else if st == svc.StopPending {
					rData.StatusText = "Stop Pending"
				} else if st == svc.Running {
					rData.StatusText = "Running"
				} else if st == svc.ContinuePending {
					rData.StatusText = "Continue Pending"
				} else if st == svc.PausePending {
					rData.StatusText = "Pause Pending"
				} else if st == svc.Paused {
					rData.StatusText = "Paused"
				}

				retData = append(retData, rData)

				iter = uintptr(unsafe.Pointer(iter + unsafe.Sizeof(sizeTest)))
			}
		} else {
			return nil, fmt.Errorf("Failed to get Service List even with allocated memory.")
		}
	} else {
		return nil, fmt.Errorf("Unable to get size of required memory allocation.")
	}
	return retData, nil
}

func StartService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func StopService(name string) error {
	return controlService(name, svc.Stop, svc.Stopped)
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(30 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}
