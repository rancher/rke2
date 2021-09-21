// +build windows,amd64

package winapi

import (
	"fmt"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	so "github.com/iamacarpet/go-win64api/shared"
)

// BackupBitLockerRecoveryKeys backups up volume recovery information to Active Directory.
// Requires one or more PersistentVolumeIDs, available from GetBitLockerRecoveryInfo.
//
// Ref: https://docs.microsoft.com/en-us/windows/win32/secprov/backuprecoveryinformationtoactivedirectory-win32-encryptablevolume
func BackupBitLockerRecoveryKeys(persistentVolumeIDs []string) error {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	w := &wmi{}
	if err := w.Connect(); err != nil {
		return fmt.Errorf("wmi.Connect: %w", err)
	}
	defer w.Close()

	for _, pvid := range persistentVolumeIDs {
		raw, err := oleutil.CallMethod(w.svc, "ExecQuery",
			fmt.Sprintf(`SELECT * FROM Win32_EncryptableVolume WHERE PersistentVolumeID="%s"`, pvid),
		)
		if err != nil {
			return fmt.Errorf("ExecQuery: %w", err)
		}
		result := raw.ToIDispatch()
		defer result.Release()

		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
		if err != nil {
			return fmt.Errorf("failed to fetch result row while processing BitLocker info: %w", err)
		}
		item := itemRaw.ToIDispatch()
		defer item.Release()

		keys, err := getKeyProtectors(item)
		if err != nil {
			return fmt.Errorf("getKeyProtectors: %w", err)
		}

		for _, k := range keys {
			statusResultRaw, err := oleutil.CallMethod(item, "BackupRecoveryInformationToActiveDirectory", k)
			if err != nil {
				return fmt.Errorf("unable to backup bitlocker information to active directory: %w", err)
			} else if val, ok := statusResultRaw.Value().(int32); val != 0 || !ok {
				return fmt.Errorf("invalid result while backing up bitlocker information to active directory: %v", val)
			}
		}
	}
	return nil
}

// GetBitLockerConversionStatus returns the Bitlocker conversion status for all local drives.
func GetBitLockerConversionStatus() ([]*so.BitLockerConversionStatus, error) {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	return getBitLockerConversionStatusInternal("")
}

// GetBitLockerConversionStatusForDrive returns the Bitlocker conversion status for a specific drive.
func GetBitLockerConversionStatusForDrive(driveLetter string) (*so.BitLockerConversionStatus, error) {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	result, err := getBitLockerConversionStatusInternal(" WHERE DriveLetter = '" + driveLetter + "'")
	if err != nil {
		return nil, err
	}

	if len(result) < 1 {
		return nil, fmt.Errorf("error getting BitLocker conversion status, drive not found: %s", driveLetter)
	} else if len(result) > 1 {
		return nil, fmt.Errorf("error getting BitLocker conversion status, too many results: %s", driveLetter)
	} else {
		return result[0], err
	}
}

// GetBitLockerRecoveryInfo returns the Bitlocker device info for all local drives.
func GetBitLockerRecoveryInfo() ([]*so.BitLockerDeviceInfo, error) {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	return getBitLockerRecoveryInfoInternal("")
}

// GetBitLockerRecoveryInfoForDrive returns the Bitlocker device info for a specific drive.
func GetBitLockerRecoveryInfoForDrive(driveLetter string) (*so.BitLockerDeviceInfo, error) {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	result, err := getBitLockerRecoveryInfoInternal(" WHERE DriveLetter = '" + driveLetter + "'")
	if err != nil {
		return nil, err
	}

	if len(result) < 1 {
		return nil, fmt.Errorf("error getting BitLocker Recovery Info, drive not found: %s", driveLetter)
	} else if len(result) > 1 {
		return nil, fmt.Errorf("error getting BitLocker Recovery Info, too many results: %s", driveLetter)
	} else {
		return result[0], err
	}
}

type wmi struct {
	intf *ole.IDispatch
	svc  *ole.IDispatch
}

func (w *wmi) Connect() error {
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return fmt.Errorf("unable to create initial object, %w", err)
	}
	defer unknown.Release()
	w.intf, err = unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("unable to create initial object, %w", err)
	}
	serviceRaw, err := oleutil.CallMethod(w.intf, "ConnectServer", nil, `\\.\ROOT\CIMV2\Security\MicrosoftVolumeEncryption`)
	if err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}
	w.svc = serviceRaw.ToIDispatch()
	return nil
}

func (w *wmi) Close() {
	w.svc.Release()
	w.intf.Release()
}

func getBitLockerConversionStatusInternal(where string) ([]*so.BitLockerConversionStatus, error) {
	w := &wmi{}
	if err := w.Connect(); err != nil {
		return nil, fmt.Errorf("wmi.Connect: %w", err)
	}
	defer w.Close()
	raw, err := oleutil.CallMethod(w.svc, "ExecQuery", "SELECT * FROM Win32_EncryptableVolume"+where)
	if err != nil {
		return nil, fmt.Errorf("ExecQuery: %w", err)
	}
	result := raw.ToIDispatch()
	defer result.Release()

	ret := []*so.BitLockerConversionStatus{}

	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return nil, fmt.Errorf("unable to get property Count while processing BitLocker info: %w", err)
	}
	count := int(countVar.Val)

	for i := 0; i < count; i++ {
		retData, err := bitlockerConversionStatus(result, i)
		if err != nil {
			return nil, err
		}

		ret = append(ret, retData)
	}

	return ret, nil
}

func getBitLockerRecoveryInfoInternal(where string) ([]*so.BitLockerDeviceInfo, error) {
	w := &wmi{}
	if err := w.Connect(); err != nil {
		return nil, fmt.Errorf("wmi.Connect: %w", err)
	}
	defer w.Close()
	raw, err := oleutil.CallMethod(w.svc, "ExecQuery", "SELECT * FROM Win32_EncryptableVolume"+where)
	if err != nil {
		return nil, fmt.Errorf("ExecQuery: %w", err)
	}
	result := raw.ToIDispatch()
	defer result.Release()

	retBitLocker := []*so.BitLockerDeviceInfo{}

	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return nil, fmt.Errorf("unable to get property Count while processing BitLocker info: %w", err)
	}
	count := int(countVar.Val)

	for i := 0; i < count; i++ {
		retData, err := bitlockerRecoveryInfo(result, i)
		if err != nil {
			return nil, err
		}

		retBitLocker = append(retBitLocker, retData)
	}

	return retBitLocker, nil
}

func getKeyProtectors(item *ole.IDispatch) ([]string, error) {
	kp := []string{}
	var keyProtectorResults ole.VARIANT
	ole.VariantInit(&keyProtectorResults)
	keyIDResultRaw, err := oleutil.CallMethod(item, "GetKeyProtectors", 3, &keyProtectorResults)
	if err != nil {
		return nil, fmt.Errorf("Unable to get Key Protectors while getting BitLocker info. %s", err.Error())
	} else if val, ok := keyIDResultRaw.Value().(int32); val != 0 || !ok {
		return nil, fmt.Errorf("Unable to get Key Protectors while getting BitLocker info. Return code %d", val)
	}
	keyProtectorValues := keyProtectorResults.ToArray().ToValueArray()
	for _, keyIDItemRaw := range keyProtectorValues {
		keyIDItem, ok := keyIDItemRaw.(string)
		if !ok {
			return nil, fmt.Errorf("KeyProtectorID wasn't a string...")
		}
		kp = append(kp, keyIDItem)
	}
	return kp, nil
}

func bitlockerConversionStatus(result *ole.IDispatch, i int) (*so.BitLockerConversionStatus, error) {
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch result row while processing BitLocker info: %w", err)
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()

	retData := &so.BitLockerConversionStatus{}

	// https://docs.microsoft.com/en-us/windows/win32/secprov/getconversionstatus-win32-encryptablevolume
	var conversionStatus ole.VARIANT
	ole.VariantInit(&conversionStatus)
	var encryptionPercentage ole.VARIANT
	ole.VariantInit(&encryptionPercentage)
	var encryptionFlags ole.VARIANT
	ole.VariantInit(&encryptionFlags)
	var wipingStatus ole.VARIANT
	ole.VariantInit(&wipingStatus)
	var wipingPercentage ole.VARIANT
	ole.VariantInit(&wipingPercentage)
	statusResultRaw, err := oleutil.CallMethod(
		item, "GetConversionStatus",
		&conversionStatus,
		&encryptionPercentage,
		&encryptionFlags,
		&wipingStatus,
		&wipingPercentage,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get conversion status while getting BitLocker info: %w", err)
	} else if val, ok := statusResultRaw.Value().(int32); val != 0 || !ok {
		return nil, fmt.Errorf("unable to get conversion status while getting BitLocker info. Return code %d", val)
	}

	retData.ConversionStatus = conversionStatus.Value().(int32)
	retData.EncryptionPercentage = encryptionPercentage.Value().(int32)
	retData.EncryptionFlags = encryptionFlags.Value().(int32)
	retData.WipingStatus = wipingStatus.Value().(int32)
	retData.WipingPercentage = wipingPercentage.Value().(int32)

	return retData, nil
}

func bitlockerRecoveryInfo(result *ole.IDispatch, i int) (*so.BitLockerDeviceInfo, error) {
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch result row while processing BitLocker info. %w", err)
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()

	retData := &so.BitLockerDeviceInfo{
		RecoveryKeys: []string{},
	}

	resDeviceID, err := oleutil.GetProperty(item, "DeviceID")
	if err != nil {
		return nil, fmt.Errorf("Error while getting property DeviceID from BitLocker info. %s", err.Error())
	}
	retData.DeviceID = resDeviceID.ToString()

	resPersistentVolumeID, err := oleutil.GetProperty(item, "PersistentVolumeID")
	if err != nil {
		return nil, fmt.Errorf("Error while getting property PersistentVolumeID from BitLocker info. %s", err.Error())
	}
	retData.PersistentVolumeID = resPersistentVolumeID.ToString()

	resDriveLetter, err := oleutil.GetProperty(item, "DriveLetter")
	if err != nil {
		return nil, fmt.Errorf("Error while getting property DriveLetter from BitLocker info. %s", err.Error())
	}
	retData.DriveLetter = resDriveLetter.ToString()

	resProtectionStatus, err := oleutil.GetProperty(item, "ProtectionStatus")
	if err != nil {
		return nil, fmt.Errorf("Error while getting property ProtectionStatus from BitLocker info. %s", err.Error())
	}
	var ok bool
	retData.ProtectionStatus, ok = resProtectionStatus.Value().(int32)
	if !ok {
		return nil, fmt.Errorf("Failed to parse ProtectionStatus from BitLocker info as uint32")
	}

	resConversionStatus, err := oleutil.GetProperty(item, "ConversionStatus")
	if err != nil {
		return nil, fmt.Errorf("error while getting property ConversionStatus from BitLocker info: %w", err)
	}
	ok = false
	retData.ConversionStatus, ok = resConversionStatus.Value().(int32)
	if !ok {
		return nil, fmt.Errorf("Failed to parse ConversionStatus from BitLocker info as uint32")
	}
	keys, err := getKeyProtectors(item)
	if err != nil {
		return nil, fmt.Errorf("getKeyProtectors: %w", err)
	}

	for _, k := range keys {
		err = func() error {
			var recoveryKey ole.VARIANT
			ole.VariantInit(&recoveryKey)
			recoveryKeyResultRaw, err := oleutil.CallMethod(item, "GetKeyProtectorNumericalPassword", k, &recoveryKey)
			if err != nil {
				return fmt.Errorf("Unable to get Recovery Key while getting BitLocker info. %s", err.Error())
			} else if val, ok := recoveryKeyResultRaw.Value().(int32); val != 0 || !ok {
				return fmt.Errorf("Unable to get Recovery Key while getting BitLocker info. Return code %d", val)
			}
			retData.RecoveryKeys = append(retData.RecoveryKeys, recoveryKey.ToString())
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	return retData, nil
}
