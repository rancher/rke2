// +build windows,amd64

package winapi

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	so "github.com/iamacarpet/go-win64api/shared"
)

func GetSystemProfile() (so.Hardware, so.OperatingSystem, so.Memory, []so.Disk, []so.Network, error) {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	retHW := so.Hardware{}
	retOS := so.OperatingSystem{}
	retMEM := so.Memory{}
	retDISK := make([]so.Disk, 0)
	retNET := make([]so.Network, 0)

	// Pre-WMI Queries
	var err error
	retHW.IsUsingUEFI, err = sysinfo_uefi_check()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, fmt.Errorf("Failed to get UEFI status, %s", err.Error())
	}
	retHW.SecureBootEnabled, err = sysinfo_secureboot_check()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, fmt.Errorf("Failed to get SecureBoot status, %s", err.Error())
	}

	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, fmt.Errorf("Unable to create initial object, %s", err.Error())
	}
	defer unknown.Release()
	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, fmt.Errorf("Unable to create query interface, %s", err.Error())
	}
	defer wmi.Release()

	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer")
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, fmt.Errorf("Error Connecting to WMI Service, %s", err.Error())
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	// Query 1 - BIOS information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT SerialNumber, Manufacturer, SMBIOSBIOSVersion, ReleaseDate FROM Win32_BIOS")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting BIOS info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing BIOS info. %s", err.Error())
		}
		count := int(countVar.Val)

		if count > 0 {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
			if err != nil {
				return fmt.Errorf("Failed to fetch result row while processing BIOS info. %s", err.Error())
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			resSerialNumber, err := oleutil.GetProperty(item, "SerialNumber")
			if err != nil {
				return fmt.Errorf("Error while getting property SerialNumber in BIOS info. %s", err.Error())
			}
			retHW.ServiceTag = resSerialNumber.ToString()
			resVersion, err := oleutil.GetProperty(item, "SMBIOSBIOSVersion")
			if err != nil {
				return fmt.Errorf("Error while getting property Version in BIOS info. %s", err.Error())
			}
			retHW.BIOSVersion = resVersion.ToString()
			resManufacturer, err := oleutil.GetProperty(item, "Manufacturer")
			if err != nil {
				return fmt.Errorf("Error while getting property Manufacturer in BIOS info. %s", err.Error())
			}
			retHW.BIOSManufacturer = resManufacturer.ToString()
			resReleaseDate, err := oleutil.GetProperty(item, "ReleaseDate")
			if err != nil {
				return fmt.Errorf("Error while getting property ReleaseDate in BIOS info. %s", err.Error())
			}
			if resReleaseDate.Value() != nil {
				if resVReleaseDate, ok := resReleaseDate.Value().(string); ok {
					resVVRD := strings.Split(resVReleaseDate, "+")[0]
					retHW.BIOSReleaseDate, err = time.Parse("20060102150405.999999", resVVRD)
					if err != nil {
						return fmt.Errorf("Unable to parse BIOS release date into valid time.Time. %s", err.Error())
					}
				} else {
					return fmt.Errorf("Unable to assert BIOS release date as string. Got type %s", reflect.TypeOf(resReleaseDate.Value()).Name())
				}
			}
		} else {
			return fmt.Errorf("Error while getting BIOS info, no BIOS record found.")
		}
		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 2 - Computer System information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT AutomaticManagedPagefile, Manufacturer, Model, TotalPhysicalMemory FROM Win32_ComputerSystem")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting Computer System info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing Computer System info. %s", err.Error())
		}
		count := int(countVar.Val)

		if count > 0 {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
			if err != nil {
				return fmt.Errorf("Failed to fetch result row while processing Computer System info. %s", err.Error())
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			resMPF, err := oleutil.GetProperty(item, "AutomaticManagedPagefile")
			if err != nil {
				return fmt.Errorf("Error while getting property AutomaticManagedPagefile in Computer System info. %s", err.Error())
			}
			if resMPF.Value() != nil {
				if resVMPF, ok := resMPF.Value().(bool); ok {
					retMEM.SystemManagedPageFile = resVMPF
				} else {
					return fmt.Errorf("Error asserting AutomaticManagedPagefile to bool. Got type %s", reflect.TypeOf(resMPF.Value()).Name())
				}
			}
			resManufacturer, err := oleutil.GetProperty(item, "Manufacturer")
			if err != nil {
				return fmt.Errorf("Error while getting property Manufacturer in Computer System info. %s", err.Error())
			}
			retHW.Manufacturer = resManufacturer.ToString()
			resModel, err := oleutil.GetProperty(item, "Model")
			if err != nil {
				return fmt.Errorf("Error while getting property Model in Computer System info. %s", err.Error())
			}
			retHW.Model = resModel.ToString()
			resTM, err := oleutil.GetProperty(item, "TotalPhysicalMemory")
			if err != nil {
				return fmt.Errorf("Error while getting property TotalPhysicalMemory in Computer System info. %s", err.Error())
			}
			if resTM.Value() != nil {
				if resVTM, ok := resTM.Value().(string); ok {
					if retMEM.TotalRAM, err = strconv.ParseUint(resVTM, 10, 64); err != nil {
						return fmt.Errorf("Error while converting TotalPhysicalMemory to integer. %s", err.Error())
					}
				} else {
					return fmt.Errorf("Error asserting TotalPhysicalMemory to string. Got type %s", reflect.TypeOf(resTM.Value()).Name())
				}
			}
		}
		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 3 - Hardware UUID information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT UUID FROM Win32_ComputerSystemProduct")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting UUID info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing UUID info. %s", err.Error())
		}
		count := int(countVar.Val)

		if count > 0 {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
			if err != nil {
				return fmt.Errorf("Failed to fetch result row while processing UUID info. %s", err.Error())
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			resUUID, err := oleutil.GetProperty(item, "UUID")
			if err != nil {
				return fmt.Errorf("Error while getting property Hardware UUID. %s", err.Error())
			}
			retHW.HardwareUUID = resUUID.ToString()
		}
		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 4 - Operating System information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Caption, Version, OSArchitecture, OSLanguage, TotalVisibleMemorySize, FreePhysicalMemory, TotalVirtualMemorySize, FreeVirtualMemory, LastBootUpTime FROM Win32_OperatingSystem")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting Operating System info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing Operating System info. %s", err.Error())
		}
		count := int(countVar.Val)

		if count > 0 {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
			if err != nil {
				return fmt.Errorf("Failed to fetch result row while processing Operating System info. %s", err.Error())
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			resFriendlyName, err := oleutil.GetProperty(item, "Caption")
			if err != nil {
				return fmt.Errorf("Error while getting property Friendly Name from Operating System info. %s", err.Error())
			}
			retOS.FriendlyName = resFriendlyName.ToString()
			resVersion, err := oleutil.GetProperty(item, "Version")
			if err != nil {
				return fmt.Errorf("Error while getting property Version from Operating System info. %s", err.Error())
			}
			retOS.Version = resVersion.ToString()
			resArch, err := oleutil.GetProperty(item, "OSArchitecture")
			if err != nil {
				return fmt.Errorf("Error while getting property Architecture from Operating System info. %s", err.Error())
			}
			retOS.Architecture = resArch.ToString()
			resLanguageCode, err := oleutil.GetProperty(item, "OSLanguage")
			if err != nil {
				return fmt.Errorf("Error while getting property Language from Language Code info. %s", err.Error())
			}
			if resLanguageCode.Value() != nil {
				if resVLC, ok := resLanguageCode.Value().(int32); ok {
					retOS.LanguageCode = uint16(resVLC)
				} else {
					return fmt.Errorf("Error while setting Lanauge Code property to int32. Got type %s", reflect.TypeOf(resLanguageCode.Value()).Name())
				}
			}

			resAvailRAM, err := oleutil.GetProperty(item, "TotalVisibleMemorySize")
			if err != nil {
				return fmt.Errorf("Error while getting property TotalVisibleMemorySize from Operating System info. %s", err.Error())
			}
			if resAvailRAM.Value() != nil {
				if resVAvailRAM, ok := resAvailRAM.Value().(string); ok {
					if retMEM.UsableRAM, err = strconv.ParseUint(resVAvailRAM, 10, 64); err != nil {
						return fmt.Errorf("Error parsing Available RAM integer. %s", err.Error())
					}
				} else {
					return fmt.Errorf("Error asserting Available RAM as string from Operating System Info. Got type %s", reflect.TypeOf(resAvailRAM.Value()).Name())
				}
			}

			resFreeRAM, err := oleutil.GetProperty(item, "FreePhysicalMemory")
			if err != nil {
				return fmt.Errorf("Error while getting property FreePhysicalMemory from Operating System info. %s", err.Error())
			}
			if resFreeRAM.Value() != nil {
				if resVFreeRAM, ok := resFreeRAM.Value().(string); ok {
					if retMEM.FreeRAM, err = strconv.ParseUint(resVFreeRAM, 10, 64); err != nil {
						return fmt.Errorf("Error parsing Free RAM integer. %s", err.Error())
					}
				} else {
					return fmt.Errorf("Error asserting Free RAM as string from Operating System Info. Got type %s", reflect.TypeOf(resFreeRAM.Value()).Name())
				}
			}

			resPageFileSize, err := oleutil.GetProperty(item, "TotalVirtualMemorySize")
			if err != nil {
				return fmt.Errorf("Error while getting property TotalVirtualMemorySize from Operating System info. %s", err.Error())
			}
			if resPageFileSize.Value() != nil {
				if resVPFS, ok := resPageFileSize.Value().(string); ok {
					if vms, err := strconv.ParseUint(resVPFS, 10, 64); err != nil {
						return fmt.Errorf("Error parsing total virtual memory integer. %s", err.Error())
					} else {
						retMEM.TotalPageFile = (vms - retMEM.UsableRAM)
					}
				} else {
					return fmt.Errorf("Error asserting TotalVirtualMemorySize as string from Operating System Info. Got type %s", reflect.TypeOf(resPageFileSize.Value()).Name())
				}
			}

			resFreePageFile, err := oleutil.GetProperty(item, "FreeVirtualMemory")
			if err != nil {
				return fmt.Errorf("Error while getting property FreeVirtualMemory from Operating System info. %s", err.Error())
			}
			if resFreePageFile.Value() != nil {
				if resVFPF, ok := resFreePageFile.Value().(string); ok {
					if fvms, err := strconv.ParseUint(resVFPF, 10, 64); err != nil {
						return fmt.Errorf("Error parsing free virtual memory integer. %s", err.Error())
					} else {
						retMEM.FreePageFile = (fvms - retMEM.FreeRAM)
					}
				} else {
					return fmt.Errorf("Error asserting FreeVirtualMemory as string from Operating System Info. Got type %s", reflect.TypeOf(resFreePageFile.Value()).Name())
				}
			}

			resLastBootUpTime, err := oleutil.GetProperty(item, "LastBootUpTime")
			if err != nil {
				return fmt.Errorf("Error while getting property LastBootUpTime from Operating System info. %s", err.Error())
			}
			if resLastBootUpTime.Value() != nil {
				if resLBUT, ok := resLastBootUpTime.Value().(string); ok {
					retOS.LastBootUpTime, err = ConvertWMITime(resLBUT)
					if err != nil {
						return fmt.Errorf("Error parsing LastBootUpTime: %s", err)
					}
				} else {
					return fmt.Errorf("Error asserting LastBootUpTime as string from Operating System Info. Got type %s", reflect.TypeOf(resFreePageFile.Value()).Name())
				}
			}
		}
		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 5 - Processor information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Name, NumberOfCores, NumberOfLogicalProcessors FROM Win32_Processor")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting Operating System info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing Operating System info. %s", err.Error())
		}
		count := int(countVar.Val)

		for i := 0; i < count; i++ {
			err = func() error {
				itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
				if err != nil {
					return fmt.Errorf("Failed to fetch result row while processing Processor info. %s", err.Error())
				}
				item := itemRaw.ToIDispatch()
				defer item.Release()

				retCPU := so.CPU{}

				resFriendlyName, err := oleutil.GetProperty(item, "Name")
				if err != nil {
					return fmt.Errorf("Error while getting property Friendly Name from Processor info. %s", err.Error())
				}
				retCPU.FriendlyName = resFriendlyName.ToString()
				resNC, err := oleutil.GetProperty(item, "NumberOfCores")
				if err != nil {
					return fmt.Errorf("Error while getting property Number of Cores from Processor info. %s", err.Error())
				}
				if resNC.Value() != nil {
					if resVNC, ok := resNC.Value().(int32); ok {
						retCPU.NumberOfCores = uint8(resVNC)
					} else {
						return fmt.Errorf("Error asserting NumberOfCores as int32 from Processor Info. Got type %s", reflect.TypeOf(resNC.Value()).Name())
					}
				}
				resNLP, err := oleutil.GetProperty(item, "NumberOfLogicalProcessors")
				if err != nil {
					return fmt.Errorf("Error while getting property Number of Logical Processors from Processor info. %s", err.Error())
				}
				if resNLP.Value() != nil {
					if resVNLP, ok := resNLP.Value().(int32); ok {
						retCPU.NumberOfLogical = uint8(resVNLP)
					} else {
						return fmt.Errorf("Error asserting NumberOfCores as int32 from Processor Info. Got type %s", reflect.TypeOf(resNLP.Value()).Name())
					}
				}

				retHW.CPU = append(retHW.CPU, retCPU)

				return nil
			}()
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 6 - Memory DIMM information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Capacity, MemoryType, Speed FROM Win32_PhysicalMemory")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting Operating System info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing Operating System info. %s", err.Error())
		}
		count := int(countVar.Val)

		for i := 0; i < count; i++ {
			err = func() error {
				itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
				if err != nil {
					return fmt.Errorf("Failed to fetch result row while processing Processor info. %s", err.Error())
				}
				item := itemRaw.ToIDispatch()
				defer item.Release()

				retMD := so.MemoryDIMM{}

				resCapacity, err := oleutil.GetProperty(item, "Capacity")
				if err != nil {
					return fmt.Errorf("Error while getting property Capacity from Memory DIMM info. %s", err.Error())
				}
				if resCapacity.Value() != nil {
					if resVCapacity, ok := resCapacity.Value().(string); ok {
						if retMD.Size, err = strconv.ParseUint(resVCapacity, 10, 64); err != nil {
							return fmt.Errorf("Error parsing Capacity integer. %s", err.Error())
						}
					} else {
						return fmt.Errorf("Error asserting Capacity as string from Memory DIMM Info. Got type %s", reflect.TypeOf(resCapacity.Value()).Name())
					}
				}
				resMT, err := oleutil.GetProperty(item, "MemoryType")
				if err != nil {
					return fmt.Errorf("Error while getting property Memory Type from Memory DIMM info. %s", err.Error())
				}
				if resMT.Value() != nil {
					if resVMT, ok := resMT.Value().(int32); ok {
						retMD.MType = memoryType[int(resVMT)]
					} else {
						return fmt.Errorf("Error asserting Memory Type as int32 from Memory DIMM Info. Got type %s", reflect.TypeOf(resMT.Value()).Name())
					}
				}
				resSpeed, err := oleutil.GetProperty(item, "Speed")
				if err != nil {
					return fmt.Errorf("Error while getting property Speed from Memory DIMM info. %s", err.Error())
				}
				if resSpeed.Value() != nil {
					if resVSpeed, ok := resSpeed.Value().(int32); ok {
						retMD.Speed = uint16(resVSpeed)
					} else {
						return fmt.Errorf("Error asserting Speed as int32 from Memory DIMM Info. Got type %s", reflect.TypeOf(resSpeed.Value()).Name())
					}
				}

				retHW.Memory = append(retHW.Memory, retMD)

				return nil
			}()
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 7 - Logical Disk information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Name, DriveType, FreeSpace, Size, FileSystem FROM Win32_LogicalDisk WHERE DriveType=3")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting Logical Disk info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing Logical Disk info. %s", err.Error())
		}
		count := int(countVar.Val)

		for i := 0; i < count; i++ {
			err = func() error {
				itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
				if err != nil {
					return fmt.Errorf("Failed to fetch result row while processing Logical Disk info. %s", err.Error())
				}
				item := itemRaw.ToIDispatch()
				defer item.Release()

				retDR := so.Disk{}

				resName, err := oleutil.GetProperty(item, "Name")
				if err != nil {
					return fmt.Errorf("Error while getting property Name from Logical Disk info. %s", err.Error())
				}
				retDR.DriveName = resName.ToString() + `\`
				resFileSystem, err := oleutil.GetProperty(item, "FileSystem")
				if err != nil {
					return fmt.Errorf("Error while getting property File System from Logical Disk info. %s", err.Error())
				}
				retDR.FileSystem = resFileSystem.ToString()

				resTotalSize, err := oleutil.GetProperty(item, "Size")
				if err != nil {
					return fmt.Errorf("Error while getting property Size from Logical Disk info. %s", err.Error())
				}
				if resTotalSize.Value() != nil {
					if resVTSize, ok := resTotalSize.Value().(string); ok {
						if retDR.TotalSize, err = strconv.ParseUint(resVTSize, 10, 64); err != nil {
							return fmt.Errorf("Error parsing Total Size (DISK) integer. %s", err.Error())
						}
					} else {
						return fmt.Errorf("Error asserting Total Size (DISK) as string from Logical Disk Info. Got type %s", reflect.TypeOf(resTotalSize.Value()).Name())
					}
				}

				resFreeSP, err := oleutil.GetProperty(item, "FreeSpace")
				if err != nil {
					return fmt.Errorf("Error while getting property FreeSpace from Logical Disk info. %s", err.Error())
				}
				if resFreeSP.Value() != nil {
					if resVAvail, ok := resFreeSP.Value().(string); ok {
						if retDR.Available, err = strconv.ParseUint(resVAvail, 10, 64); err != nil {
							return fmt.Errorf("Error parsing Available (DISK) integer. %s", err.Error())
						}
					} else {
						return fmt.Errorf("Error asserting Available (DISK) as string from Logical Disk Info. Got type %s", reflect.TypeOf(resFreeSP.Value()).Name())
					}
				}

				retDR.BitLockerEnabled, retDR.BitLockerEncrypted, _ = sysinfo_bitlocker_check(resName.ToString())

				retDR.BitLockerRecoveryInfo, _ = GetBitLockerRecoveryInfoForDrive(resName.ToString())

				retDISK = append(retDISK, retDR)

				return nil
			}()
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	// Query 8 - Network Adapter information.
	err = func() error {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Description, IPAddress, MACAddress, IPSubnet, DHCPEnabled FROM Win32_NetworkAdapterConfiguration WHERE MACAddress <> NULL")
		if err != nil {
			return fmt.Errorf("Unable to execute query while getting Network Adapter info. %s", err.Error())
		}
		result := resultRaw.ToIDispatch()
		defer result.Release()

		countVar, err := oleutil.GetProperty(result, "Count")
		if err != nil {
			return fmt.Errorf("Unable to get property Count while processing Network Adapter info. %s", err.Error())
		}
		count := int(countVar.Val)

		for i := 0; i < count; i++ {
			err = func() error {
				itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
				if err != nil {
					return fmt.Errorf("Failed to fetch result row while processing Network Adapter info. %s", err.Error())
				}
				item := itemRaw.ToIDispatch()
				defer item.Release()

				retNW := so.Network{}

				resName, err := oleutil.GetProperty(item, "Description")
				if err != nil {
					return fmt.Errorf("Error while getting property Description from Network Adapter info. %s", err.Error())
				}
				retNW.Name = resName.ToString()

				resDHCP, err := oleutil.GetProperty(item, "DHCPEnabled")
				if err != nil {
					return fmt.Errorf("Error while getting property DHCPEnabled from Network Adapter info. %s", err.Error())
				}
				var ok bool
				if resDHCP.Value() != nil {
					if retNW.DHCPEnabled, ok = resDHCP.Value().(bool); !ok {
						return fmt.Errorf("Error while asserting type bool for DHCPEnabled at Network Adapter info. Got type %s", reflect.TypeOf(resDHCP.Value()).Name())
					}
				}

				resMAC, err := oleutil.GetProperty(item, "MACAddress")
				if err != nil {
					return fmt.Errorf("Error while getting property MAC Address from Network Adapter info. %s", err.Error())
				}
				retNW.MACAddress = resMAC.ToString()

				var ips []interface{}
				var subnets []interface{}
				resIP, err := oleutil.GetProperty(item, "IPAddress")
				if err != nil {
					return fmt.Errorf("Error while getting property IP Address from Network Adapter info. %s", err.Error())
				}
				if uintptr(resIP.Val) != 0 && resIP.VT != ole.VT_NULL {
					ips = resIP.ToArray().ToValueArray()
				}
				resSubnet, err := oleutil.GetProperty(item, "IPSubnet")
				if err != nil {
					return fmt.Errorf("Error while getting property IP Subnet from Network Adapter info. %s", err.Error())
				}
				if uintptr(resSubnet.Val) != 0 && resSubnet.VT != ole.VT_NULL {
					subnets = resSubnet.ToArray().ToValueArray()
				}

				for i, v := range ips {
					var ip, subnet string
					if ip, ok = v.(string); !ok {
						return fmt.Errorf("Unable to assert IP as string. Got type %s", reflect.TypeOf(v).Name())
					}
					if subnet, ok = subnets[i].(string); !ok {
						return fmt.Errorf("Unable to assert Subnet as string. Got type %s", reflect.TypeOf(subnets[i]).Name())
					}
					if strings.Contains(subnet, ".") {
						tsubnet := ParseIPv4Mask(subnet)
						tIP := net.ParseIP(ip)
						tIPNet := net.IPNet{IP: tIP, Mask: tsubnet}
						ip = tIPNet.String()
					} else {
						tsbits, err := strconv.Atoi(subnet)
						if err != nil {
							return fmt.Errorf("Unable to convert IPv6 Subnet to integer.")
						}
						tsubnet := net.CIDRMask(tsbits, 128)
						tIP := net.ParseIP(ip)
						tIPNet := net.IPNet{IP: tIP, Mask: tsubnet}
						ip = tIPNet.String()
					}

					retNW.IPAddressCIDR = append(retNW.IPAddressCIDR, ip)
				}

				retNET = append(retNET, retNW)

				return nil
			}()
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return retHW, retOS, retMEM, retDISK, retNET, err
	}

	//fmt.Printf("%#v\r\n%#v\r\n%#v\r\n%#v\r\n%#v\r\n", retHW, retOS, retMEM, retDISK, retNET)

	return retHW, retOS, retMEM, retDISK, retNET, nil
}

var memoryType = map[int]string{
	0:  "Unknown",
	1:  "Other",
	2:  "DRAM",
	3:  "Synch DRAM",
	4:  "Cache DRAM",
	5:  "EDO",
	6:  "EDRAM",
	7:  "VRAM",
	8:  "SRAM",
	9:  "RAM",
	10: "ROM",
	11: "Flash",
	12: "EEPROM",
	13: "FEPROM",
	14: "EPROM",
	15: "CDRAM",
	16: "3DRAM",
	17: "SDRAM",
	18: "SGRAM",
	19: "RDRAM",
	20: "DDR",
	21: "DDR2",
	22: "DDR2 FB-DIMM",
	24: "DDR3",
	25: "FBD2",
	26: "DDR4",
	27: "LPDDR",
	28: "LPDDR2",
	29: "LPDDR3",
	30: "LPDDR4",
}

func ParseIPv4Mask(s string) net.IPMask {
	mask := net.ParseIP(s)
	if mask == nil {
		return nil
	}
	return net.IPv4Mask(mask[12], mask[13], mask[14], mask[15])
}

func ConvertWMITime(s string) (time.Time, error) {
	if len(s) == 26 {
		return time.Time{}, fmt.Errorf("Invalid Length of DATETIME string")
	}

	offset := s[22:]
	iOffset, err := strconv.Atoi(offset)
	if err != nil {
		return time.Time{}, fmt.Errorf("Error parsing offset: %s", err)
	}

	var h, m int = (iOffset / 60), (iOffset % 60)
	var hr, mn string
	if h < 10 {
		hr = "0" + strconv.Itoa(h)
	} else {
		hr = strconv.Itoa(h)
	}
	if m < 10 {
		mn = "0" + strconv.Itoa(m)
	} else {
		mn = strconv.Itoa(m)
	}

	res, err := time.Parse("20060102150405.999999-07:00", s[:22]+hr+":"+mn)
	if err != nil {
		return time.Time{}, fmt.Errorf("Error parsing time: %s", err)
	}

	return res, nil
}
