package shared

// BitLockerConversionStatus represents the GetConversionStatus method of Win32_EncryptableVolume
type BitLockerConversionStatus struct {
	ConversionStatus     int32
	EncryptionPercentage int32
	EncryptionFlags      int32
	WipingStatus         int32
	WipingPercentage     int32
}

// BitLockerDeviceInfo contains the bitlocker state for a given device
type BitLockerDeviceInfo struct {
	DeviceID           string
	PersistentVolumeID string
	DriveLetter        string
	ProtectionStatus   int32
	ConversionStatus   int32
	RecoveryKeys       []string
}
