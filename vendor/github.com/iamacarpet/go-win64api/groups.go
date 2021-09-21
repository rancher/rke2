// +build windows,amd64

package winapi

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	so "github.com/iamacarpet/go-win64api/shared"
)

var (
	usrNetLocalGroupAdd        = modNetapi32.NewProc("NetLocalGroupAdd")
	usrNetLocalGroupEnum       = modNetapi32.NewProc("NetLocalGroupEnum")
	usrNetLocalGroupDel        = modNetapi32.NewProc("NetLocalGroupDel")
	usrNetLocalGroupSetMembers = modNetapi32.NewProc("NetLocalGroupSetMembers")
	usrNetLocalGroupGetMembers = modNetapi32.NewProc("NetLocalGroupGetMembers")
)

// Possible errors returned by local group management functions
// Error code enumerations taken from MS-ERREF documentation:
// https://msdn.microsoft.com/en-us/library/cc231196.aspx
const (
	NERR_GroupNotFound syscall.Errno = 2220 // 0x000008AC

	ERROR_ACCESS_DENIED       syscall.Errno = 5    // 0x00000005
	ERROR_MEMBER_NOT_IN_ALIAS syscall.Errno = 1377 // 0x00000561
	ERROR_MEMBER_IN_ALIAS     syscall.Errno = 1378 // 0x00000562
	ERROR_NO_SUCH_MEMBER      syscall.Errno = 1387 // 0x0000056B
	ERROR_INVALID_MEMBER      syscall.Errno = 1388 // 0x0000056C
)

// LOCALGROUP_INFO_0 represents level 0 information about local Windows groups.
// This struct matches the struct definition in the Windows headers (lmaccess.h).
type LOCALGROUP_INFO_0 struct {
	Lgrpi0_name *uint16 // UTF-16 group name
}

// LOCALGROUP_INFO_1 represents level 1 information about local Windows groups.
// This struct matches the struct definition in the Windows headers (lmaccess.h).
type LOCALGROUP_INFO_1 struct {
	Lgrpi1_name    *uint16 // UTF-16 group name
	Lgrpi1_comment *uint16 // UTF-16 group comment
}

// LocalGroupAdd adds a new local group with the specified name and comment.
func LocalGroupAdd(name, comment string) (bool, error) {
	var parmErr uint32
	var err error
	var gInfo LOCALGROUP_INFO_1

	gInfo.Lgrpi1_name, err = syscall.UTF16PtrFromString(name)
	if err != nil {
		return false, fmt.Errorf("Unable to encode group name to UTF16: %s", err)
	}
	gInfo.Lgrpi1_comment, err = syscall.UTF16PtrFromString(comment)
	if err != nil {
		return false, fmt.Errorf("Unable to encode comment to UTF16: %s", err)
	}

	ret, _, _ := usrNetLocalGroupAdd.Call(
		uintptr(0),                        // server name
		uintptr(uint32(1)),                // information level
		uintptr(unsafe.Pointer(&gInfo)),   // group information
		uintptr(unsafe.Pointer(&parmErr)), // error code out param
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, syscall.Errno(ret)
	}
	return true, nil
}

// ListLocalGroups enumerates the local groups defined on the system.
//
// If an error occurs in the call to the underlying NetLocalGroupEnum function, the
// returned error will be a syscall.Errno containing the error code.
// See: https://docs.microsoft.com/en-us/windows/desktop/api/lmaccess/nf-lmaccess-netlocalgroupenum
func ListLocalGroups() ([]so.LocalGroup, error) {
	var (
		dataPointer  uintptr
		resumeHandle uintptr
		entriesRead  uint32
		entriesTotal uint32
		sizeTest     LOCALGROUP_INFO_1
		retVal       = make([]so.LocalGroup, 0)
	)

	ret, _, _ := usrNetLocalGroupEnum.Call(
		uintptr(0),                                 // servername
		uintptr(uint32(1)),                         // level, LOCALGROUP_INFO_1
		uintptr(unsafe.Pointer(&dataPointer)),      // struct buffer for output data.
		uintptr(uint32(USER_MAX_PREFERRED_LENGTH)), // allow as much memory as required.
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&entriesTotal)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return nil, syscall.Errno(ret)
	} else if dataPointer == uintptr(0) {
		return nil, fmt.Errorf("null pointer while fetching entry")
	}
	defer usrNetApiBufferFree.Call(dataPointer)

	var iter = dataPointer
	for i := uint32(0); i < entriesRead; i++ {
		var data = (*LOCALGROUP_INFO_1)(unsafe.Pointer(iter))

		gd := so.LocalGroup{
			Name:    UTF16toString(data.Lgrpi1_name),
			Comment: UTF16toString(data.Lgrpi1_comment),
		}
		retVal = append(retVal, gd)

		iter = uintptr(unsafe.Pointer(iter + unsafe.Sizeof(sizeTest)))
	}

	return retVal, nil
}

// LocalGroupDel deletes the specified local group.
//
// If an error occurs in the call to the underlying NetLocalGroupDel function, the
// returned error will be a syscall.Errno containing the error code.
// See: https://docs.microsoft.com/en-us/windows/desktop/api/lmaccess/nf-lmaccess-netlocalgroupdel
func LocalGroupDel(name string) (bool, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false, fmt.Errorf("Unable to encode group name to UTF16: %s", err)
	}

	ret, _, _ := usrNetLocalGroupDel.Call(
		uintptr(0), // servername
		uintptr(unsafe.Pointer(namePtr)),
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, syscall.Errno(ret)
	}
	return true, nil
}

func localGroupModMembers(proc *syscall.LazyProc, groupname string, usernames []string) (bool, error) {
	memberInfos := make([]LOCALGROUP_MEMBERS_INFO_3, 0, len(usernames))
	hostname, err := os.Hostname()
	if err != nil {
		return false, fmt.Errorf("Unable to determine hostname: %s", err)
	}
	groupnamePtr, err := syscall.UTF16PtrFromString(groupname)
	if err != nil {
		return false, fmt.Errorf("Unable to encode group name to UTF16: %s", err)
	}

	for _, username := range usernames {
		domainAndUsername := username
		if !strings.ContainsRune(username, '\\') {
			domainAndUsername = fmt.Sprintf(`%s\%s`, hostname, username)
		}
		namePtr, err := syscall.UTF16PtrFromString(domainAndUsername)
		if err != nil {
			return false, fmt.Errorf("Unable to encode username to UTF16: %s", err)
		}
		memberInfos = append(memberInfos, LOCALGROUP_MEMBERS_INFO_3{
			Lgrmi3_domainandname: namePtr,
		})
	}

	if len(memberInfos) == 0 {
		// Add a fake entry just so that the slice isn't empty, so we can take
		// the address of the first entry
		memberInfos = append(memberInfos, LOCALGROUP_MEMBERS_INFO_3{})
	}

	ret, _, _ := proc.Call(
		uintptr(0),                               // servername
		uintptr(unsafe.Pointer(groupnamePtr)),    // group name
		uintptr(3),                               // level, LOCALGROUP_MEMBERS_INFO_3
		uintptr(unsafe.Pointer(&memberInfos[0])), // buf
		uintptr(len(usernames)),                  // totalEntries
	)
	if ret != NET_API_STATUS_NERR_Success {
		return false, syscall.Errno(ret)
	}

	return true, nil
}

// LocalGroupSetMembers sets the membership of the group to contain exactly the
// set of users specified in usernames.
//
// If an error occurs in the call to the underlying NetLocalGroupSetMembers function, the
// returned error will be a syscall.Errno containing the error code.
// See: https://docs.microsoft.com/en-us/windows/desktop/api/lmaccess/nf-lmaccess-netlocalgroupsetmembers
func LocalGroupSetMembers(groupname string, usernames []string) (bool, error) {
	return localGroupModMembers(usrNetLocalGroupSetMembers, groupname, usernames)
}

// LocalGroupAddMembers adds the specified members to the group, if they are not
// already members.
//
// If an error occurs in the call to the underlying NetLocalGroupAddMembers function, the
// returned error will be a syscall.Errno containing the error code.
// See: https://docs.microsoft.com/en-us/windows/desktop/api/lmaccess/nf-lmaccess-netlocalgroupaddmembers
func LocalGroupAddMembers(groupname string, usernames []string) (bool, error) {
	return localGroupModMembers(usrNetLocalGroupAddMembers, groupname, usernames)
}

// LocalGroupDelMembers removes the specified members from the local group.
//
// If an error occurs in the call to the underlying NetLocalGroupDelMembers function, the
// returned error will be a syscall.Errno containing the error code.
// See: https://docs.microsoft.com/en-us/windows/desktop/api/lmaccess/nf-lmaccess-netlocalgroupdelmembers
func LocalGroupDelMembers(groupname string, usernames []string) (bool, error) {
	return localGroupModMembers(usrNetLocalGroupDelMembers, groupname, usernames)
}

// LocalGroupGetMembers returns information about the members of the specified
// local group.
//
// If an error occurs in the call to the underlying NetLocalGroupGetMembers function, the
// returned error will be a syscall.Errno containing the error code.
// See: https://docs.microsoft.com/en-us/windows/desktop/api/lmaccess/nf-lmaccess-netlocalgroupgetmembers
func LocalGroupGetMembers(groupname string) ([]so.LocalGroupMember, error) {
	var (
		dataPointer  uintptr
		resumeHandle uintptr
		entriesRead  uint32
		entriesTotal uint32
		sizeTest     LOCALGROUP_MEMBERS_INFO_3
		retVal       []so.LocalGroupMember = make([]so.LocalGroupMember, 0)
	)

	groupnamePtr, err := syscall.UTF16PtrFromString(groupname)
	if err != nil {
		return nil, fmt.Errorf("Unable to encode group name to UTF16: %s", err)
	}

	ret, _, _ := usrNetLocalGroupGetMembers.Call(
		uintptr(0),                                 // servername
		uintptr(unsafe.Pointer(groupnamePtr)),      // group name
		uintptr(3),                                 // level, LOCALGROUP_MEMBERS_INFO_3
		uintptr(unsafe.Pointer(&dataPointer)),      // bufptr
		uintptr(uint32(USER_MAX_PREFERRED_LENGTH)), // prefmaxlen
		uintptr(unsafe.Pointer(&entriesRead)),      // entriesread
		uintptr(unsafe.Pointer(&entriesTotal)),     // totalentries
		uintptr(unsafe.Pointer(&resumeHandle)),     // resumehandle
	)
	if ret != NET_API_STATUS_NERR_Success {
		return nil, syscall.Errno(ret)
	} else if dataPointer == uintptr(0) {
		return nil, fmt.Errorf("null pointer while fetching entry")
	}
	defer usrNetApiBufferFree.Call(dataPointer)

	var iter = dataPointer
	for i := uint32(0); i < entriesRead; i++ {
		var data = (*LOCALGROUP_MEMBERS_INFO_3)(unsafe.Pointer(iter))

		domainAndUsername := UTF16toString(data.Lgrmi3_domainandname)
		split := strings.SplitN(domainAndUsername, "\\", 1)
		var domain, name string
		if len(split) > 1 {
			domain = split[0]
			name = split[1]
		} else {
			// This really shouldn't happen, but just in case...
			name = split[0]
		}

		gd := so.LocalGroupMember{
			Domain:        domain,
			Name:          name,
			DomainAndName: domainAndUsername,
		}
		retVal = append(retVal, gd)

		iter = uintptr(unsafe.Pointer(iter + unsafe.Sizeof(sizeTest)))
	}

	return retVal, nil
}
