# Windows EventLog hooks for Logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/>

## Usage

```go
import (
  "log/syslog"
  "github.com/sirupsen/logrus"
  "github.com/Freman/eventloghook"
  "golang.org/x/sys/windows/svc/eventlog"
)

func main() {
  log       := logrus.New()
  elog, err = eventlog.Open("Service Name")
  if err != nil {
    panic(err)
  }
  defer elog.Close()
  log.Hooks.Add(eventloghook.NewHook(elog))
}
```

If you want to output to the windows console/terminal - tho why you'd want to do that vs use logrus built in method I have no idea :D

```go
import (
  "log/syslog"
  "github.com/Sirupsen/logrus"
  "github.com/Freman/eventloghook"
  "golang.org/x/sys/windows/svc/debug"
)

func main() {
  log       := logrus.New()
  elog = debug.New("Service Name")
  defer elog.Close()
  log.Hooks.Add(eventloghook.NewHook(elog))
}
```
