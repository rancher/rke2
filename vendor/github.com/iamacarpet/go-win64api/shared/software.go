package shared

import "time"

// EstimatedSize is in KB,
// As estimated & written to the registry by the installer itself,
// or Windows Installer for an MSI.
type Software struct {
	DisplayName     string    `json:"displayName"`
	DisplayVersion  string    `json:"displayVersion"`
	Arch            string    `json:"arch"`
	Publisher       string    `json:"publisher"`
	InstallDate     time.Time `json:"installDate"`
	EstimatedSize   uint64    `json:"estimatedSize"`
	Contact         string    `json:"Contact"`
	HelpLink        string    `json:"HelpLink"`
	InstallSource   string    `json:"InstallSource"`
	InstallLocation string    `json:"InstallLocation"`
	UninstallString string    `json:"UninstallString"`
	VersionMajor    uint64    `json:"VersionMajor"`
	VersionMinor    uint64    `json:"VersionMinor"`
}

func (s *Software) Name() string {
	return s.DisplayName
}

func (s *Software) Version() string {
	return s.DisplayVersion
}

func (s *Software) Architecture() string {
	return s.Arch
}
