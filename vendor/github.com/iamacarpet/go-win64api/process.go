// +build windows,amd64

package winapi

import (
	"fmt"
	"reflect"
	"syscall"
	"unsafe"

	so "github.com/iamacarpet/go-win64api/shared"
)

// Windows API functions
var (
	modKernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procCloseHandle               = modKernel32.NewProc("CloseHandle")
	procOpenProcess               = modKernel32.NewProc("OpenProcess")
	procCreateToolhelp32Snapshot  = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First            = modKernel32.NewProc("Process32FirstW")
	procProcess32Next             = modKernel32.NewProc("Process32NextW")
	procQueryFullProcessImageName = modKernel32.NewProc("QueryFullProcessImageNameW")
	procGetCurrentProcess         = modKernel32.NewProc("GetCurrentProcess")
	procTerminateProcess          = modKernel32.NewProc("TerminateProcess")
	procGetLastError              = modKernel32.NewProc("GetLastError")
	procSetThreadExecutionState   = modKernel32.NewProc("SetThreadExecutionState")

	modAdvapi32                  = syscall.NewLazyDLL("advapi32.dll")
	procOpenProcessToken         = modAdvapi32.NewProc("OpenProcessToken")
	procLookupPrivilegeValue     = modAdvapi32.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges    = modAdvapi32.NewProc("AdjustTokenPrivileges")
	procGetTokenInformation      = modAdvapi32.NewProc("GetTokenInformation")
	procLookupAccountSid         = modAdvapi32.NewProc("LookupAccountSidW")
	procCheckTokenMembership     = modAdvapi32.NewProc("CheckTokenMembership")
	procAllocateAndInitializeSid = modAdvapi32.NewProc("AllocateAndInitializeSid")
	procFreeSid                  = modAdvapi32.NewProc("FreeSid")
	procDuplicateToken           = modAdvapi32.NewProc("DuplicateToken")
)

// Some constants from the Windows API
const (
	ERROR_NO_MORE_FILES               = 0x12
	PROCESS_TERMINATE                 = 0x0001
	PROCESS_QUERY_INFORMATION         = 0x0400
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	MAX_PATH                          = 260
	MAX_FULL_PATH                     = 4096

	ES_AWAYMODE_REQUIRED = 0x00000040
	ES_CONTINUOUS        = 0x80000000
	ES_DISPLAY_REQUIRED  = 0x00000002
	ES_SYSTEM_REQUIRED   = 0x00000001
	ES_USER_PRESENT      = 0x00000004

	PROC_TOKEN_DUPLICATE         = 0x0002
	PROC_TOKEN_QUERY             = 0x0008
	PROC_TOKEN_ADJUST_PRIVILEGES = 0x0020

	PROC_SE_PRIVILEGE_ENABLED = 0x00000002

	PROC_SE_DEBUG_NAME              = "SeDebugPrivilege"
	PROC_SE_SYSTEM_ENVIRONMENT_PRIV = "SeSystemEnvironmentPrivilege"

	PROC_SECURITY_BUILTIN_DOMAIN_RID = 0x00000020
	PROC_DOMAIN_ALIAS_RID_ADMINS     = 0x00000220

	PROC_ERROR_NO_SUCH_LOGON_SESSION = 1312
	PROC_ERROR_PRIVILEGE_NOT_HELD    = 1314
)

// PROCESSENTRY32 is the Windows API structure that contains a process's
// information.
type PROCESSENTRY32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [MAX_PATH]uint16
}

type TOKEN_PRIVILEGES struct {
	PrivilegeCount uint32
	Privileges     [1]LUID_AND_ATTRIBUTES
}

type LUID_AND_ATTRIBUTES struct {
	LUID       LUID
	Attributes uint32
}

type TOKEN_USER struct {
	User SID_AND_ATTRIBUTES
}

type SID_AND_ATTRIBUTES struct {
	Sid        uintptr
	Attributes uint32
}

type TOKEN_STATISTICS struct {
	TokenId            LUID
	AuthenticationId   LUID
	ExpirationTime     uint64
	TokenType          uint32
	ImpersonationLevel uint32
	DynamicCharged     uint32
	DynamicAvailable   uint32
	GroupCount         uint32
	PrivilegeCount     uint32
	ModifiedId         LUID
}

type PSID uintptr

type SID_IDENTIFIER_AUTHORITY struct {
	Value [6]byte
}

func newProcessData(e *PROCESSENTRY32, path string, user string) so.Process {
	// Find when the string ends for decoding
	end := 0
	for {
		if e.ExeFile[end] == 0 {
			break
		}
		end++
	}

	return so.Process{
		Pid:        int(e.ProcessID),
		Ppid:       int(e.ParentProcessID),
		Executable: syscall.UTF16ToString(e.ExeFile[:end]),
		Fullpath:   path,
		Username:   user,
	}
}

func SetThreadExecutionState(state uint32) (uint32, error) {
	res, _, err := procSetThreadExecutionState.Call(uintptr(state))
	if err != nil {
		return 0, err
	}

	return uint32(res), nil
}

func ProcessKill(pid uint32) (bool, error) {
	handle, _, _ := procOpenProcess.Call(uintptr(uint32(PROCESS_TERMINATE)), uintptr(0), uintptr(pid))
	if handle < 0 {
		return false, fmt.Errorf("Failed to open handle: %s", syscall.GetLastError())
	}
	defer procCloseHandle.Call(handle)

	res, _, _ := procTerminateProcess.Call(handle, uintptr(uint32(0)))
	if res != 1 {
		return false, fmt.Errorf("Failed to terminate process!")
	}

	return true, nil
}

func ProcessList() ([]so.Process, error) {
	err := procAssignCorrectPrivs(PROC_SE_DEBUG_NAME)
	if err != nil {
		return nil, fmt.Errorf("Error assigning privs... %s", err.Error())
	}

	lList, err := sessUserLUIDs()
	if err != nil {
		return nil, fmt.Errorf("Error getting LUIDs... %s", err.Error())
	}

	handle, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002, 0)
	if handle < 0 {
		return nil, syscall.GetLastError()
	}
	defer procCloseHandle.Call(handle)

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("Error retrieving process info.")
	}

	results := make([]so.Process, 0)
	for {
		path, ll, _ := getProcessFullPathAndLUID(entry.ProcessID)

		var user string
		for k, l := range lList {
			if reflect.DeepEqual(k, ll) {
				user = l
				break
			}
		}

		results = append(results, newProcessData(&entry, path, user))

		ret, _, _ := procProcess32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return results, nil
}

type SessionLUID struct {
	Value   LUID
	IsAdmin bool
}

func ProcessLUIDList() (map[uint32]SessionLUID, error) {
	err := procAssignCorrectPrivs(PROC_SE_DEBUG_NAME)
	if err != nil {
		return nil, fmt.Errorf("Error assigning privs... %s", err.Error())
	}

	handle, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002, 0)
	if handle < 0 {
		return nil, syscall.GetLastError()
	}
	defer procCloseHandle.Call(handle)

	pMap := make(map[uint32]SessionLUID)

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("Error retrieving process info.")
	}

	for {
		ll, isAdmin, _ := getProcessLUID(entry.ProcessID)

		pMap[entry.ProcessID] = SessionLUID{
			Value:   ll,
			IsAdmin: isAdmin,
		}

		ret, _, _ := procProcess32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return pMap, nil
}

func procAssignCorrectPrivs(name string) error {
	handle, _, _ := procGetCurrentProcess.Call()
	if handle == uintptr(0) {
		return fmt.Errorf("Unable to get current process handle.")
	}
	defer procCloseHandle.Call(handle)

	var tHandle uintptr
	opRes, _, _ := procOpenProcessToken.Call(
		uintptr(handle),
		uintptr(uint32(PROC_TOKEN_ADJUST_PRIVILEGES)),
		uintptr(unsafe.Pointer(&tHandle)),
	)
	if opRes != 1 {
		return fmt.Errorf("Unable to open current process token.")
	}
	defer procCloseHandle.Call(tHandle)

	nPointer, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("Unable to encode SE_DEBUG_NAME to UTF16")
	}
	var pValue LUID
	lpRes, _, _ := procLookupPrivilegeValue.Call(
		uintptr(0),
		uintptr(unsafe.Pointer(nPointer)),
		uintptr(unsafe.Pointer(&pValue)),
	)
	if lpRes != 1 {
		return fmt.Errorf("Unable to lookup priv value.")
	}

	iVal := TOKEN_PRIVILEGES{
		PrivilegeCount: 1,
	}
	iVal.Privileges[0] = LUID_AND_ATTRIBUTES{
		LUID:       pValue,
		Attributes: PROC_SE_PRIVILEGE_ENABLED,
	}
	ajRes, _, _ := procAdjustTokenPrivileges.Call(
		uintptr(tHandle),
		uintptr(uint32(0)),
		uintptr(unsafe.Pointer(&iVal)),
		uintptr(uint32(0)),
		uintptr(0),
		uintptr(0),
	)
	if ajRes != 1 {
		return fmt.Errorf("Error while adjusting process token.")
	}
	return nil
}

func getProcessLUID(pid uint32) (retLUID LUID, isAdmin bool, retError error) {
	retLUID = LUID{}

	handle, _, lastError := procOpenProcess.Call(uintptr(uint32(PROCESS_QUERY_INFORMATION)), uintptr(0), uintptr(pid))
	if handle < 0 {
		retError = lastError
		return
	}
	defer procCloseHandle.Call(handle)

	var ptHandle uintptr
	opRes, _, _ := procOpenProcessToken.Call(
		uintptr(handle),
		uintptr(uint32(PROC_TOKEN_QUERY)),
		uintptr(unsafe.Pointer(&ptHandle)),
	)
	if opRes != 1 {
		retError = fmt.Errorf("Unable to open process token.")
		return
	}
	defer procCloseHandle.Call(ptHandle)

	var sData TOKEN_STATISTICS
	var sLength uint32
	tsRes, _, _ := procGetTokenInformation.Call(
		uintptr(ptHandle),
		uintptr(uint32(10)), // TOKEN_STATISTICS
		uintptr(unsafe.Pointer(&sData)),
		uintptr(uint32(unsafe.Sizeof(sData))),
		uintptr(unsafe.Pointer(&sLength)),
	)
	if tsRes != 1 {
		retError = fmt.Errorf("Error fetching token information (LUID).")
		return
	}
	retLUID = sData.AuthenticationId

	// Grab the token again for the next bit,
	// as if we try and combine this with the first bit,
	// the token grab will fail unless run as NT AUTHORITY\SYSTEM
	var tHandle uintptr
	opRes2, _, _ := procOpenProcessToken.Call(
		uintptr(handle),
		uintptr(uint32(PROC_TOKEN_QUERY|PROC_TOKEN_DUPLICATE)),
		uintptr(unsafe.Pointer(&tHandle)),
	)
	if opRes2 != 1 {
		retError = fmt.Errorf("Unable to open process token.")
		return
	}
	defer procCloseHandle.Call(tHandle)

	// Generate an SID for the Administrators group.
	NtAuthority := SID_IDENTIFIER_AUTHORITY{
		Value: [6]byte{0, 0, 0, 0, 0, 5}, // SECURITY_NT_AUTHORITY
	}
	var AdministratorsGroup PSID
	sidRes, _, _ := procAllocateAndInitializeSid.Call(
		uintptr(unsafe.Pointer(&NtAuthority)),
		uintptr(uint8(2)),
		uintptr(uint32(PROC_SECURITY_BUILTIN_DOMAIN_RID)),
		uintptr(uint32(PROC_DOMAIN_ALIAS_RID_ADMINS)),
		uintptr(0), uintptr(0), uintptr(0), uintptr(0), uintptr(0), uintptr(0),
		uintptr(unsafe.Pointer(&AdministratorsGroup)),
	)
	if sidRes != 1 {
		retError = fmt.Errorf("Error generating Administrators SID for group membership check")
		return
	}
	defer procFreeSid.Call(uintptr(AdministratorsGroup))

	// Duplicate the token...
	var ttHandle uintptr
	dupRes, _, lastError := procDuplicateToken.Call(
		uintptr(tHandle),
		uintptr(1), // _SECURITY_IMPERSONATION_LEVEL = SecurityIdentification
		uintptr(unsafe.Pointer(&ttHandle)),
	)
	defer procCloseHandle.Call(ttHandle)
	if dupRes != 1 {
		retError = fmt.Errorf("Error generating impersonation token: %s", lastError)
		return
	}

	// Check token for membership of the Administrators group.
	var chkRes bool
	mbrRes, _, _ := procCheckTokenMembership.Call(
		uintptr(ttHandle),
		uintptr(AdministratorsGroup),
		uintptr(unsafe.Pointer(&chkRes)),
	)
	if mbrRes != 1 {
		retError = fmt.Errorf("Error checking group membership")
		return
	} else {
		isAdmin = chkRes
	}

	var ltHandle uintptr
	var length uint32
	ltokRes, _, lastError := procGetTokenInformation.Call(
		uintptr(tHandle),
		uintptr(19), // TokenLinkedToken
		uintptr(unsafe.Pointer(&ltHandle)),
		uintptr(uint32(unsafe.Sizeof(ltHandle))),
		uintptr(unsafe.Pointer(&length)),
	)
	if ltokRes != 1 {
		if lastError.(syscall.Errno) == PROC_ERROR_NO_SUCH_LOGON_SESSION || lastError.(syscall.Errno) == PROC_ERROR_PRIVILEGE_NOT_HELD {
			return
		} else {
			retError = fmt.Errorf("Error getting linked token: %d: %s", lastError.(syscall.Errno), lastError)
			return
		}
	}
	defer procCloseHandle.Call(ltHandle)

	var lttHandle uintptr
	dup2Res, _, lastError := procDuplicateToken.Call(
		uintptr(ltHandle),
		uintptr(1), // _SECURITY_IMPERSONATION_LEVEL = SecurityIdentification
		uintptr(unsafe.Pointer(&lttHandle)),
	)
	if dup2Res != 1 {
		retError = fmt.Errorf("Error generating impersonation token (2): %s", lastError)
		return
	}
	defer procCloseHandle.Call(lttHandle)

	mbr2Res, _, lastError := procCheckTokenMembership.Call(
		uintptr(lttHandle),
		uintptr(AdministratorsGroup),
		uintptr(unsafe.Pointer(&chkRes)),
	)
	if mbr2Res != 1 {
		retError = fmt.Errorf("Error checking group membership (2): %s", lastError)
		return
	} else {
		isAdmin = chkRes
	}

	return
}

func getProcessFullPathAndLUID(pid uint32) (string, LUID, error) {
	var fullpath string

	handle, _, _ := procOpenProcess.Call(uintptr(uint32(PROCESS_QUERY_INFORMATION)), uintptr(0), uintptr(pid))
	if handle < 0 {
		return "", LUID{}, syscall.GetLastError()
	}
	defer procCloseHandle.Call(handle)

	var pathName [MAX_FULL_PATH]uint16
	pathLength := uint32(MAX_FULL_PATH)
	ret, _, _ := procQueryFullProcessImageName.Call(handle, uintptr(0), uintptr(unsafe.Pointer(&pathName)), uintptr(unsafe.Pointer(&pathLength)))

	if ret > 0 {
		fullpath = syscall.UTF16ToString(pathName[:pathLength])
	}

	var tHandle uintptr
	opRes, _, _ := procOpenProcessToken.Call(
		uintptr(handle),
		uintptr(uint32(PROC_TOKEN_QUERY)),
		uintptr(unsafe.Pointer(&tHandle)),
	)
	if opRes != 1 {
		return fullpath, LUID{}, fmt.Errorf("Unable to open process token.")
	}
	defer procCloseHandle.Call(tHandle)

	var sData TOKEN_STATISTICS
	var sLength uint32
	tsRes, _, _ := procGetTokenInformation.Call(
		uintptr(tHandle),
		uintptr(uint32(10)), // TOKEN_STATISTICS
		uintptr(unsafe.Pointer(&sData)),
		uintptr(uint32(unsafe.Sizeof(sData))),
		uintptr(unsafe.Pointer(&sLength)),
	)
	if tsRes != 1 {
		return fullpath, LUID{}, fmt.Errorf("Error fetching token information (LUID).")
	}

	return fullpath, sData.AuthenticationId, nil
}
