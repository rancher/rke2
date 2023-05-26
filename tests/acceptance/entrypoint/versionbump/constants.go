package versionbump

var (
	ExpectedValueUpgradedHost string
	ExpectedValueUpgradedNode string
	CmdHost                   string
	ExpectedValueHost         string
	CmdNode                   string
	ExpectedValueNode         string
	Description               string
	GetRuncVersion            = "(find /var/lib/rancher/rke2/data/ -type f -name runc -exec {} --version \\;)"
)
