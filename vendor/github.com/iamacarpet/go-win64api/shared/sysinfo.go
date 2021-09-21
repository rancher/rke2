package shared

import (
	"time"
)

type Hardware struct {
	HardwareUUID      string       `json:"HardwareUUID"`
	Manufacturer      string       `json:"Manufacturer"`
	Model             string       `json:"Model"`
	ServiceTag        string       `json:"ServiceTag"`
	BIOSVersion       string       `json:"biosVersion"`
	BIOSManufacturer  string       `json:"biosManufacturer"`
	BIOSReleaseDate   time.Time    `json:"biosReleaseDate"`
	IsUsingUEFI       bool         `json:"isUsingUEFI"`
	SecureBootEnabled bool         `json:"safebootEnabled"`
	CPU               []CPU        `json:"cpus"`
	Memory            []MemoryDIMM `json:"memoryDIMMs"`
}

type CPU struct {
	FriendlyName    string `json:"FriendlyName"`
	NumberOfCores   uint8  `json:"cores"`
	NumberOfLogical uint8  `json:"logical"`
}

type MemoryDIMM struct {
	MType string `json:"MemoryType"`
	Size  uint64 `json:"Size"`
	Speed uint16 `json:"Speed"`
}

type OperatingSystem struct {
	FriendlyName   string    `json:"FriendlyName"`
	Version        string    `json:"Version"`
	Architecture   string    `json:"Architecture"`
	LanguageCode   uint16    `json:"Language"`
	LastBootUpTime time.Time `json:"LastBootUpTime`
}

type Memory struct {
	TotalRAM              uint64 `json:"totalRAM"`
	UsableRAM             uint64 `json:"usableRAM"`
	FreeRAM               uint64 `json:"freeRAM"`
	TotalPageFile         uint64 `json:"totalPF"`
	FreePageFile          uint64 `json:"freePF"`
	SystemManagedPageFile bool   `json:"managedPF"`
}

type Disk struct {
	DriveName             string               `json:"DriveName"`
	TotalSize             uint64               `json:"TotalSize"`
	Available             uint64               `json:"FreeSpace"`
	FileSystem            string               `json:"FileSystem"`
	BitLockerEnabled      bool                 `json:"BitLockerEnabled"`
	BitLockerEncrypted    bool                 `json:"BitLockerEncrypted"`
	BitLockerRecoveryInfo *BitLockerDeviceInfo `json:"BitLockerRecoveryInfo,omitempty"`
}

type Network struct {
	Name          string   `json:"NetworkName"`
	MACAddress    string   `json:"MACAddress"`
	IPAddressCIDR []string `json:"IPAddresses"`
	DHCPEnabled   bool     `json:"DHCPEnabled"`
}
