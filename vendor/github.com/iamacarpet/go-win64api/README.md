# GoLang Windows API Wrappers
## For System Info / User Management.
For an internal project, this is a set of wrappers for snippets of the Windows API.

Tested and developed for Windows 10 x64.

All functions that return useful data, do so in the form of JSON exportable structs.

These structs are available in the shared library, "github.com/iamacarpet/go-win64api/shared"

### Process List
```go
package main

import (
    "fmt"
    wapi "github.com/iamacarpet/go-win64api"
)

func main(){
    pr, err := wapi.ProcessList()
    if err != nil {
        fmt.Printf("Error fetching process list... %s\r\n", err.Error())
    }
    for _, p := range pr {
        fmt.Printf("%8d - %-30s - %-30s - %s\r\n", p.Pid, p.Username, p.Executable, p.Fullpath)
    }
}
```

### Active Session List (Logged in users + Run-As users)
```go
package main

import (
    "fmt"
    wapi "github.com/iamacarpet/go-win64api"
)

func main(){
    // This check runs best as NT AUTHORITY\SYSTEM
    //
    // Running as a normal or even elevated user,
    // we can't properly detect who is an admin or not.
    //
    // This is because we require TOKEN_DUPLICATE permission,
    // which we don't seem to have otherwise (Win10).
    users, err := wapi.ListLoggedInUsers()
    if err != nil {
        fmt.Printf("Error fetching user session list.\r\n")
        return
    }

    fmt.Printf("Users currently logged in (Admin check doesn't work for AD Accounts):\r\n")
    for _, u := range users {
        fmt.Printf("\t%-50s - Local User: %-5t - Local Admin: %t\r\n", u.FullUser(), u.LocalUser, u.LocalAdmin)
    }
}
```

### Installed Software List
```go
package main

import (
    "fmt"
    wapi "github.com/iamacarpet/go-win64api"
)

func main(){
    sw, err := wapi.InstalledSoftwareList()
    if err != nil {
        fmt.Printf("%s\r\n", err.Error())
    }

    for _, s := range sw {
        fmt.Printf("%-100s - %s - %s\r\n", s.Name(), s.Architecture(), s.Version())
    }
}
```

### Windows Update Status
```go
package main

import (
        "fmt"
        "time"
        wapi "github.com/iamacarpet/go-win64api"
)

func main() {
        ret, err := wapi.UpdatesPending()
        if err != nil {
                fmt.Printf("Error fetching data... %s\r\n", err.Error())
        }

        fmt.Printf("Number of Updates Available: %d\n", ret.NumUpdates)
        fmt.Printf("Updates Pending:             %t\n\n", ret.UpdatesReq)
        fmt.Printf("%25s | %25s | %s\n", "EVENT DATE", "STATUS", "UPDATE NAME")
        for _, v := range ret.UpdateHistory {
                fmt.Printf("%25s | %25s | %s\n", v.EventDate.Format(time.RFC822), v.Status, v.UpdateName)
        }
}
```

## Local Service Management
### List Services
```go
package main

import (
    "fmt"

    wapi "github.com/iamacarpet/go-win64api"
)

func main(){
    svc, err := wapi.GetServices()
    if err != nil {
        fmt.Printf("%s\r\n", err.Error())
    }

    for _, v := range svc {
        fmt.Printf("%-50s - %-75s - Status: %-20s - Accept Stop: %-5t, Running Pid: %d\r\n", v.SCName, v.DisplayName, v.StatusText, v.AcceptStop, v.RunningPid)
    }
}
```
### Start Service
```go
err := wapi.StartService(service_name)
```
### Stop Service
```go
err := wapi.StopService(service_name)
```

## Local User Management
### List Local Users
```go
package main

import (
    "fmt"
    "time"
    wapi "github.com/iamacarpet/go-win64api"
)

func main(){
    users, err := wapi.ListLocalUsers()
    if err != nil {
        fmt.Printf("Error fetching user list, %s.\r\n", err.Error())
        return
    }

    for _, u := range users {
        fmt.Printf("%s (%s)\r\n", u.Username, u.FullName)
        fmt.Printf("\tIs Enabled:                   %t\r\n", u.IsEnabled)
        fmt.Printf("\tIs Locked:                    %t\r\n", u.IsLocked)
        fmt.Printf("\tIs Admin:                     %t\r\n", u.IsAdmin)
        fmt.Printf("\tPassword Never Expires:       %t\r\n", u.PasswordNeverExpires)
        fmt.Printf("\tUser can't change password:   %t\r\n", u.NoChangePassword)
        fmt.Printf("\tPassword Age:                 %.0f days\r\n", (u.PasswordAge.Hours()/24))
        fmt.Printf("\tLast Logon Time:              %s\r\n", u.LastLogon.Format(time.RFC850))
        fmt.Printf("\tBad Password Count:           %d\r\n", u.BadPasswordCount)
        fmt.Printf("\tNumber Of Logons:             %d\r\n", u.NumberOfLogons)
    }
}
```
### Adding a Local User
```go
ok, err := wapi.UserAdd(username, fullname, password)
```
### Deleting a Local User
```go
ok, err := wapi.UserDelete(username)
```
### Set Full Name Attribute
```go
ok, err := wapi.UserUpdateFullname(username, fullname)
```
### Give Admin Privileges
```go
ok, err := wapi.SetAdmin(username)
```
### Revoke Admin Privileges
```go
ok, err := wapi.RevokeAdmin(username)
```
### Disable/Enable User
```go
s := true   // disable user
s := false  // enable user
ok, err := wapi.UserDisabled(username, s)
```
### Change Attribute - User Can't Change Password
```go
s := true   // User can't change password
s := false  // User can change password
ok, err := wapi.UserDisablePasswordChange(username, s)
```
### Change Attribute - Password Never Expires
```go
s := true   // Password never expires.
s := false  // Enable password expiry.
ok, err := wapi.UserPasswordNoExpires(username, s)
```
### Forced Password Change
```go
ok, err := wapi.ChangePassword(username, newpassword)
```

### Windows Firewall - Add Inbound Rule
```go
added, err := wapi.FirewallRuleCreate(
	"App Rule Name",
	"App Rule Long Description.",
	"My Rule Group",
	"%systemDrive%\\path\\to\\my.exe",
	"port number as string",
	wapi.NET_FW_IP_PROTOCOL_TCP,
)
```
