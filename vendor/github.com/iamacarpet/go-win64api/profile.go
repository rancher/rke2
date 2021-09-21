// +build windows,amd64

package winapi

import (
	"syscall"
	"unsafe"
)

var (
	modUserenv                          = syscall.NewLazyDLL("Userenv.dll")
	procGetDefaultUserProfileDirectoryW = modUserenv.NewProc("GetDefaultUserProfileDirectoryW")
	procGetProfilesDirectoryW           = modUserenv.NewProc("GetProfilesDirectoryW")
)

// GetDefaultUserProfileDirectory returns the path to the directory in which the
// default user's profile is stored.
//
// See: https://docs.microsoft.com/en-us/windows/desktop/api/userenv/nf-userenv-getdefaultuserprofiledirectoryw
func GetDefaultUserProfileDirectory() (string, error) {
	var bufferSize uint32

	r1, _, err := procGetDefaultUserProfileDirectoryW.Call(
		uintptr(0),                           // lpProfileDir = NULL,
		uintptr(unsafe.Pointer(&bufferSize)), // lpcchSize = &bufferSize
	)
	// The first call always "fails" due to the buffer being NULL, but it should
	// have stored the needed buffer size in the variable bufferSize.

	// Sanity check to make sure bufferSize is sane.
	if bufferSize == 0 {
		return "", err
	}

	// bufferSize now contains the size of the buffer needed to contain the path.
	buffer := make([]uint16, bufferSize)
	r1, _, err = procGetDefaultUserProfileDirectoryW.Call(
		uintptr(unsafe.Pointer(&buffer[0])),  // lpProfileDir = &buffer
		uintptr(unsafe.Pointer(&bufferSize)), // lpcchSize = &bufferSize
	)
	if r1 == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buffer), nil
}

// GetProfilesDirectory returns the path to the directory in which user profiles
// are stored. Profiles for new users are stored in subdirectories.
//
// See: https://docs.microsoft.com/en-us/windows/desktop/api/userenv/nf-userenv-getprofilesdirectoryw
func GetProfilesDirectory() (string, error) {
	var bufferSize uint32

	r1, _, err := procGetProfilesDirectoryW.Call(
		uintptr(0),                           // lpProfileDir = NULL,
		uintptr(unsafe.Pointer(&bufferSize)), // lpcchSize = &bufferSize
	)
	// The first call always "fails" due to the buffer being NULL, but it should
	// have stored the needed buffer size in the variable bufferSize.

	// Sanity check to make sure bufferSize is sane.
	if bufferSize == 0 {
		return "", err
	}

	// bufferSize now contains the size of the buffer needed to contain the path.
	buffer := make([]uint16, bufferSize)
	r1, _, err = procGetProfilesDirectoryW.Call(
		uintptr(unsafe.Pointer(&buffer[0])),  // lpProfileDir = &buffer
		uintptr(unsafe.Pointer(&bufferSize)), // lpcchSize = &bufferSize
	)
	if r1 == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buffer), nil
}
