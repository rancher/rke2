// +build windows,amd64

package winapi

import (
	"fmt"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var (
	krnGetFirmwareEnvironmentVariable = modKernel32.NewProc("GetFirmwareEnvironmentVariableW")
)

const (
	ERROR_INVALID_FUNCTION = 1
)

func sysinfo_uefi_check() (bool, error) {
	blankStringPtr, _ := syscall.UTF16PtrFromString("")
	blankUUIDPtr, _ := syscall.UTF16PtrFromString("{00000000-0000-0000-0000-000000000000}")
	_, _, err := krnGetFirmwareEnvironmentVariable.Call(uintptr(unsafe.Pointer(blankStringPtr)), uintptr(unsafe.Pointer(blankUUIDPtr)), uintptr(0), uintptr(uint32(0)))
	if val, ok := err.(syscall.Errno); !ok {
		return false, fmt.Errorf("Unknown Error! - %s", err)
	} else if val == ERROR_INVALID_FUNCTION {
		return false, nil
	} else {
		return true, nil
	}
}

func sysinfo_secureboot_check() (bool, error) {
	procAssignCorrectPrivs(PROC_SE_SYSTEM_ENVIRONMENT_PRIV)

	nameStrPtr, _ := syscall.UTF16PtrFromString("SecureBoot")
	UUIDStrPtr, _ := syscall.UTF16PtrFromString("{8be4df61-93ca-11d2-aa0d-00e098032b8c}")
	var secureBootResult bool
	_, _, err := krnGetFirmwareEnvironmentVariable.Call(uintptr(unsafe.Pointer(nameStrPtr)), uintptr(unsafe.Pointer(UUIDStrPtr)), uintptr(unsafe.Pointer(&secureBootResult)), uintptr(uint32(unsafe.Sizeof(secureBootResult))))
	if val, ok := err.(syscall.Errno); !ok {
		return false, fmt.Errorf("Unknown Error! - %s", err)
	} else if val == ERROR_INVALID_FUNCTION {
		// BIOS - SecureBoot Unsupported
		return false, nil
	} else {
		if secureBootResult {
			return true, nil
		} else {
			return false, nil
		}
	}
}

// return enabled, encrypted, error
func sysinfo_bitlocker_check(driveName string) (bool, bool, error) {
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return false, false, fmt.Errorf("Unable to create initial object, %s", err.Error())
	}
	defer unknown.Release()
	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return false, false, fmt.Errorf("Unable to create initial object, %s", err.Error())
	}
	defer wmi.Release()
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", nil, `\\.\ROOT\CIMV2\Security\MicrosoftVolumeEncryption`)
	if err != nil {
		return false, false, fmt.Errorf("Permission Denied - %s", err)
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT ConversionStatus FROM Win32_EncryptableVolume WHERE DriveLetter = '"+driveName+"'")
	if err != nil {
		return false, false, fmt.Errorf("Unable to execute query while getting BitLocker status. %s", err.Error())
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return false, false, fmt.Errorf("Unable to get property Count while processing BitLocker status. %s", err.Error())
	}
	count := int(countVar.Val)

	if count > 0 {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
		if err != nil {
			return false, false, fmt.Errorf("Failed to fetch result row while processing BitLocker status. %s", err.Error())
		}
		item := itemRaw.ToIDispatch()
		defer item.Release()

		resStatus, err := oleutil.GetProperty(item, "ConversionStatus")
		if err != nil {
			return false, false, fmt.Errorf("Error while getting property ConversionStatus in BitLocker Status. %s", err.Error())
		}
		if status, ok := resStatus.Value().(int32); ok {
			if status == 0 {
				return false, false, nil
			} else if status == 1 {
				return true, true, nil
			} else {
				return true, false, nil
			}
		} else {
			return false, false, fmt.Errorf("Unable to convert status to uint32 in BitLocker Status")
		}
	} else {
		return false, false, nil
	}
}
