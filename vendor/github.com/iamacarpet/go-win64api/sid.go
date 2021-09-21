// +build windows,amd64

package winapi

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	usrLookupAccountNameW     = modAdvapi32.NewProc("LookupAccountNameW")
	usrConvertSidToStringSidW = modAdvapi32.NewProc("ConvertSidToStringSidW")

	usrLocalFree = modKernel32.NewProc("LocalFree")
)

// GetRawSidForAccountName looks up the SID for a given account name using the
// LookupAccountNameW system call.
// The SID is returned as a buffer containing the raw _SID struct.
//
// See: https://docs.microsoft.com/en-us/windows/desktop/api/winbase/nf-winbase-lookupaccountnamew
func GetRawSidForAccountName(accountName string) ([]byte, error) {
	if !strings.ContainsRune(accountName, '\\') {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to lookup hostname while fully qualifying account name: %v", err)
		}
		accountName = hostname + "\\" + accountName
	}

	namePointer, err := syscall.UTF16PtrFromString(accountName)
	if err != nil {
		return nil, fmt.Errorf("failed to convert account name to UTF16: %v", err)
	}

	var sidSize uint32
	var refDomainSize uint32
	var eUse byte

	// Get sizes first, which always returns failure.
	_, _, err = usrLookupAccountNameW.Call(
		uintptr(0),                              // servername
		uintptr(unsafe.Pointer(namePointer)),    // account name
		uintptr(0),                              // SID
		uintptr(unsafe.Pointer(&sidSize)),       // SID buffer size
		uintptr(0),                              // referenced domain
		uintptr(unsafe.Pointer(&refDomainSize)), // referenced domain buffer size
		uintptr(unsafe.Pointer(&eUse)),          // Account type enumeration
	)

	// Check the sizes to make sure they're sane
	if sidSize == 0 || refDomainSize == 0 {
		return nil, fmt.Errorf("LookupAccountNameW reported 0 buffer size: %v", err)
	}

	sidBuffer := make([]byte, sidSize)
	refDomain := make([]uint16, refDomainSize)

	// Call for real this time
	r1, _, err := usrLookupAccountNameW.Call(
		uintptr(0),                              // servername
		uintptr(unsafe.Pointer(namePointer)),    // account name
		uintptr(unsafe.Pointer(&sidBuffer[0])),  // SID
		uintptr(unsafe.Pointer(&sidSize)),       // SID buffer size
		uintptr(unsafe.Pointer(&refDomain[0])),  // referenced domain
		uintptr(unsafe.Pointer(&refDomainSize)), // referenced domain buffer size
		uintptr(unsafe.Pointer(&eUse)),          // Account type enumeration
	)

	// LookupAccountNameW returns non-zero on success
	if r1 == 0 {
		return nil, err
	}

	return sidBuffer, nil
}

// ConvertRawSidToStringSid converts a buffer containing a raw _SID struct
// (like what is returned by GetRawSidForAccountName) into a string SID.
//
// See: https://docs.microsoft.com/en-us/windows/desktop/api/sddl/nf-sddl-convertsidtostringsidw
func ConvertRawSidToStringSid(rawSid []byte) (string, error) {
	if len(rawSid) < 8 {
		// 8 bytes is the minimum valid size for an _SID struct if there are 0
		// sub authorities.
		// revision (1 byte) + # sub authorities (1 byte) + identifier authority (6 bytes)
		return "", fmt.Errorf("Invalid SID: buffer too short, expected at least 8 bytes, got %d", len(rawSid))
	}

	var sidStringPtr uintptr

	r1, _, err := usrConvertSidToStringSidW.Call(
		uintptr(unsafe.Pointer(&rawSid[0])),
		uintptr(unsafe.Pointer(&sidStringPtr)),
	)
	// ConvertSidtoStringSidW returns non-zero on success.
	if r1 == 0 {
		return "", err
	}
	if sidStringPtr == 0 {
		return "", fmt.Errorf("ConvertSidToStringW returned null pointer")
	}

	defer usrLocalFree.Call(sidStringPtr)

	return UTF16toString((*uint16)(unsafe.Pointer(sidStringPtr))), nil
}
