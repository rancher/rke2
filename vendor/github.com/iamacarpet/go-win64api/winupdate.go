// +build windows,amd64

package winapi

import (
	"fmt"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	so "github.com/iamacarpet/go-win64api/shared"
)

var updateResultStatus []string = []string{
	"Pending",
	"In Progress",
	"Completed",
	"Completed With Errors",
	"Failed",
	"Aborted",
}

func UpdatesPending() (*so.WindowsUpdate, error) {
	retData := &so.WindowsUpdate{}

	ole.CoInitialize(0)
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("Microsoft.Update.Session")
	if err != nil {
		return nil, fmt.Errorf("Unable to create initial object, %s", err.Error())
	}
	defer unknown.Release()
	update, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("Unable to create query interface, %s", err.Error())
	}
	defer update.Release()
	oleutil.PutProperty(update, "ClientApplicationID", "GoLang Windows API")

	us, err := oleutil.CallMethod(update, "CreateUpdateSearcher")
	if err != nil {
		return nil, fmt.Errorf("Error creating update searcher, %s", err.Error())
	}
	usd := us.ToIDispatch()
	defer usd.Release()

	usr, err := oleutil.CallMethod(usd, "Search", "IsInstalled=0 and Type='Software' and IsHidden=0")
	if err != nil {
		return nil, fmt.Errorf("Error performing update search, %s", err.Error())
	}
	usrd := usr.ToIDispatch()
	defer usrd.Release()

	upd, err := oleutil.GetProperty(usrd, "Updates")
	if err != nil {
		return nil, fmt.Errorf("Error getting Updates collection, %s", err.Error())
	}
	updd := upd.ToIDispatch()
	defer updd.Release()

	updn, err := oleutil.GetProperty(updd, "Count")
	if err != nil {
		return nil, fmt.Errorf("Error getting update count, %s", err.Error())
	}
	retData.NumUpdates = int(updn.Val)

	thc, err := oleutil.CallMethod(usd, "GetTotalHistoryCount")
	if err != nil {
		return nil, fmt.Errorf("Error getting update history count, %s", err.Error())
	}
	thcn := int(thc.Val)

	uhistRaw, err := oleutil.CallMethod(usd, "QueryHistory", 0, thcn)
	if err != nil {
		return nil, fmt.Errorf("Error querying update history, %s", err.Error())
	}
	uhist := uhistRaw.ToIDispatch()
	defer uhist.Release()

	countUhist, err := oleutil.GetProperty(uhist, "Count")
	if err != nil {
		return nil, fmt.Errorf("Unable to get property Count while processing Windows Update history: %s", err.Error())
	}
	count := int(countUhist.Val)

	for i := 0; i < count; i++ {
		err = func() error {
			itemRaw, err := oleutil.GetProperty(uhist, "Item", i)
			if err != nil {
				return fmt.Errorf("Failed to fetch result row while processing Windows Update history. %s", err.Error())
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			updateName, err := oleutil.GetProperty(item, "Title")
			if err != nil {
				return fmt.Errorf("Error while getting property Title from Windows Update history. %s", err.Error())
			}

			updateDate, err := oleutil.GetProperty(item, "Date")
			if err != nil {
				return fmt.Errorf("Error while getting property Title from Windows Update history. %s", err.Error())
			}

			resultCode, err := oleutil.GetProperty(item, "ResultCode")
			if err != nil {
				return fmt.Errorf("Error while getting property Title from Windows Update history. %s", err.Error())
			}

			retData.UpdateHistory = append(retData.UpdateHistory, &so.WindowsUpdateHistory{
				EventDate:  updateDate.Value().(time.Time),
				Status:     updateResultStatus[int(resultCode.Val)],
				UpdateName: updateName.Value().(string),
			})

			return nil
		}()
		if err != nil {
			return nil, fmt.Errorf("Unable to process update history entry: %d. %s", i, err)
		}
	}

	if retData.NumUpdates > 0 {
		retData.UpdatesReq = true
	}

	return retData, nil
}
