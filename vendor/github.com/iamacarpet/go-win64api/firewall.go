// +build windows,amd64

package winapi

import (
	"fmt"
	"runtime"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Firewall related API constants.
const (
	NET_FW_IP_PROTOCOL_TCP    = 6
	NET_FW_IP_PROTOCOL_UDP    = 17
	NET_FW_IP_PROTOCOL_ICMPv4 = 1
	NET_FW_IP_PROTOCOL_ICMPv6 = 58
	NET_FW_IP_PROTOCOL_ANY    = 256

	NET_FW_RULE_DIR_IN  = 1
	NET_FW_RULE_DIR_OUT = 2

	NET_FW_ACTION_BLOCK = 0
	NET_FW_ACTION_ALLOW = 1

	// NET_FW_PROFILE2_CURRENT is not real API constant, just helper used in FW functions.
	// It can mean one profile or multiple (even all) profiles. It depends on which profiles
	// are currently in use. Every active interface can have it's own profile. F.e.: Public for Wifi,
	// Domain for VPN, and Private for LAN. All at the same time.
	NET_FW_PROFILE2_CURRENT = 0
	NET_FW_PROFILE2_DOMAIN  = 1
	NET_FW_PROFILE2_PRIVATE = 2
	NET_FW_PROFILE2_PUBLIC  = 4
	NET_FW_PROFILE2_ALL     = 2147483647
)

// Firewall Rule Groups
// Use this magical strings instead of group names. It will work on all language Windows versions.
// You can find more string locations here:
// https://windows10dll.nirsoft.net/firewallapi_dll.html
const (
	NET_FW_FILE_AND_PRINTER_SHARING = "@FirewallAPI.dll,-28502"
	NET_FW_REMOTE_DESKTOP           = "@FirewallAPI.dll,-28752"
)

// FWProfiles represents currently active Firewall profile(-s).
type FWProfiles struct {
	Domain, Private, Public bool
}

// FWRule represents Firewall Rule.
type FWRule struct {
	Name, Description, ApplicationName, ServiceName string
	LocalPorts, RemotePorts                         string
	// LocalAddresses, RemoteAddresses are always returned with netmask, f.e.:
	//   `10.10.1.1/255.255.255.0`
	LocalAddresses, RemoteAddresses string
	// ICMPTypesAndCodes is string. You can find define multiple codes separated by ":" (colon).
	// Types are listed here:
	// https://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml
	// So to allow ping set it to:
	//   "0"
	ICMPTypesAndCodes string
	Grouping          string
	// InterfaceTypes can be:
	//   "LAN", "Wireless", "RemoteAccess", "All"
	// You can add multiple deviding with comma:
	//   "LAN, Wireless"
	InterfaceTypes                        string
	Protocol, Direction, Action, Profiles int32
	Enabled, EdgeTraversal                bool
}

// InProfiles returns FWProfiles struct, so You
// can check in which Profiles rule is active.
//
// As alternative You can analyze FWRule.Profile value.
func (r *FWRule) InProfiles() FWProfiles {
	if r.Profiles == NET_FW_PROFILE2_ALL {
		return FWProfiles{true, true, true}
	}
	return firewallParseProfiles(r.Profiles)
}

// FirewallRuleAdd creates Inbound rule for given port or ports.
//
// Rule Name is mandatory and must not contain the "|" character.
//
// Description and Group are optional. Description also can not contain the "|" character.
//
// Port(-s) is mandatory.
// Ports string can look like:
//   "5800, 5900, 6810-6812"
//
// Protocol will usually be:
//   NET_FW_IP_PROTOCOL_TCP
//   // or
//   NET_FW_IP_PROTOCOL_UDP
//
// Profile will decide in which profiles rule will apply. You can use:
//   NET_FW_PROFILE2_CURRENT // adds rule to currently used FW Profile(-s)
//   NET_FW_PROFILE2_ALL // adds rule to all profiles
//   NET_FW_PROFILE2_DOMAIN|NET_FW_PROFILE2_PRIVATE // rule in Private and Domain profile
func FirewallRuleAdd(name, description, group, ports string, protocol, profile int32) (bool, error) {

	if ports == "" {
		return false, fmt.Errorf("empty FW Rule ports, it is mandatory")
	}
	return firewallRuleAdd(name, description, group, "", "", ports, "", "", "", "", protocol, 0, profile, true, false)
}

// FirewallRuleAddApplication creates Inbound rule for given application.
//
// Rule Name is mandatory and must not contain the "|" character.
//
// Description and Group are optional. Description also can not contain the "|" character.
//
// AppPath is mandatory.
// AppPath string should look like:
//   `%ProgramFiles% (x86)\RemoteControl\winvnc.exe`
// Protocol will usually be:
//   NET_FW_IP_PROTOCOL_TCP
//   // or
//   NET_FW_IP_PROTOCOL_UDP
//
// Profile will decide in which profiles rule will apply. You can use:
//   NET_FW_PROFILE2_CURRENT // adds rule to currently used FW Profile
//   NET_FW_PROFILE2_ALL // adds rule to all profiles
//   NET_FW_PROFILE2_DOMAIN|NET_FW_PROFILE2_PRIVATE // rule in Private and Domain profile
func FirewallRuleAddApplication(name, description, group, appPath string, profile int32) (bool, error) {
	if appPath == "" {
		return false, fmt.Errorf("empty FW Rule appPath, it is mandatory")
	}
	return firewallRuleAdd(name, description, group, appPath, "", "", "", "", "", "", 0, 0, profile, true, false)
}

// FirewallRuleCreate is deprecated, use FirewallRuleAddApplication instead.
func FirewallRuleCreate(name, description, group, appPath, port string, protocol int32) (bool, error) {
	return firewallRuleAdd(name, description, group, appPath, "", port, "", "", "", "", protocol, 0, NET_FW_PROFILE2_CURRENT, true, false)
}

// FirewallPingEnable creates Inbound ICMPv4 rule which allows to answer echo requests.
//
// Rule Name is mandatory and must not contain the "|" character.
//
// Description and Group are optional. Description also can not contain the "|" character.
//
// RemoteAddresses allows you to limit pinging to f.e.:
//   "10.10.10.0/24"
// This will be internally converted to:
//   "10.10.10.0/255.255.255.0"
//
// Profile will decide in which profiles rule will apply. You can use:
//   NET_FW_PROFILE2_CURRENT // adds rule to currently used FW Profile
//   NET_FW_PROFILE2_ALL // adds rule to all profiles
//   NET_FW_PROFILE2_DOMAIN|NET_FW_PROFILE2_PRIVATE // rule in Private and Domain profile
func FirewallPingEnable(name, description, group, remoteAddresses string, profile int32) (bool, error) {
	return firewallRuleAdd(name, description, group, "", "", "", "", "", remoteAddresses, "8:*", NET_FW_IP_PROTOCOL_ICMPv4, 0, profile, true, false)
}

// FirewallRuleAddAdvanced allows to modify almost all available FW Rule parameters.
// You probably do not want to use this, as function allows to create any rule, even opening all ports
// in given profile. So use with caution.
//
// HINT: Use FirewallRulesGet to get examples how rules can be defined.
func FirewallRuleAddAdvanced(rule FWRule) (bool, error) {
	return firewallRuleAdd(rule.Name, rule.Description, rule.Grouping, rule.ApplicationName, rule.ServiceName,
		rule.LocalPorts, rule.RemotePorts, rule.LocalAddresses, rule.RemoteAddresses, rule.ICMPTypesAndCodes,
		rule.Protocol, rule.Direction, rule.Profiles, rule.Enabled, rule.EdgeTraversal)
}

// FirewallRuleDelete allows you to delete existing rule by name.
// If multiple rules with the same name exists, first (random?) is
// deleted. You can run this function in loop if You want to remove
// all of them:
//   var err error
//   for {
//       if ok, err := wapi.FirewallRuleDelete("anydesk.exe"); !ok || err != nil {
//           break
//       }
//    }
//    if err != nil {
//        fmt.Println(err)
//    }
func FirewallRuleDelete(name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("empty FW Rule name, name is mandatory")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return false, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	unknownRules, err := oleutil.GetProperty(fwPolicy, "Rules")
	if err != nil {
		return false, fmt.Errorf("Failed to get Rules: %s", err)
	}
	rules := unknownRules.ToIDispatch()

	if ok, err := FirewallRuleExistsByName(rules, name); err != nil {
		return false, fmt.Errorf("Error while checking rules for duplicate: %s", err)
	} else if !ok {
		return false, nil
	}

	if _, err := oleutil.CallMethod(rules, "Remove", name); err != nil {
		return false, fmt.Errorf("Error removing Rule: %s", err)
	}

	return true, nil
}

// FirewallRuleGet returns firewall rule by given name.
// Error is returned if there is problem calling API.
//
// If rule is not found, no error is returned, so check:
//  if len(returnedRule) == 0 {
//      if err != nil {
//          fmt.Println(err)
//      } else {
//          fmt.Println("rule not found")
//      }
//  }
func FirewallRuleGet(name string) (FWRule, error) {
	var rule FWRule

	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return rule, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	ur, ep, enum, err := firewallRulesEnum(fwPolicy)
	if err != nil {
		return rule, err
	}
	defer firewallRulesEnumRealease(ur, ep)

	for itemRaw, length, err := enum.Next(1); length > 0; itemRaw, length, err = enum.Next(1) {
		if err != nil {
			return rule, fmt.Errorf("failed to seek next Rule item: %s", err)
		}
		item := itemRaw.ToIDispatch()
		n, err := firewallRuleName(item)
		if err != nil {
			return rule, err
		}
		if name == n {
			rule, err = firewallRuleParams(itemRaw)
			if err != nil {
				return rule, err
			}
		} else {
			// only not matching rules can be released
			item.Release()
		}
	}

	return rule, nil
}

// FirewallRulesGet returns all rules defined in firewall.
func FirewallRulesGet() ([]FWRule, error) {
	rules := make([]FWRule, 1000)

	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return rules, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	ur, ep, enum, err := firewallRulesEnum(fwPolicy)
	if err != nil {
		return rules, err
	}
	defer firewallRulesEnumRealease(ur, ep)

	for itemRaw, length, err := enum.Next(1); length > 0; itemRaw, length, err = enum.Next(1) {
		if err != nil {
			return rules, fmt.Errorf("failed to seek next Rule item: %s", err)
		}

		rule, err := firewallRuleParams(itemRaw)
		if err != nil {
			return rules, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func firewallRuleName(item *ole.IDispatch) (string, error) {

	name, err := oleutil.GetProperty(item, "Name")
	if err != nil {
		return "", fmt.Errorf("failed to get Property (Name) of Rule, err: %v", err)
	}
	return name.ToString(), nil
}

// firewallRuleParams retrieves all Rule parameters from API and saves them in FWRule struct.
func firewallRuleParams(itemRaw ole.VARIANT) (FWRule, error) {
	var rule FWRule
	item := itemRaw.ToIDispatch()
	defer item.Release()

	name, err := oleutil.GetProperty(item, "Name")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Name) of Rule")
	}
	rule.Name = name.ToString()
	description, err := oleutil.GetProperty(item, "Description")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Description) of Rule %q", rule.Name)
	}
	rule.Description = description.ToString()
	applicationApplicationName, err := oleutil.GetProperty(item, "ApplicationName")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (ApplicationName) of Rule %q", rule.Name)
	}
	rule.ApplicationName = applicationApplicationName.ToString()
	serviceName, err := oleutil.GetProperty(item, "ServiceName")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (ServiceName) of Rule %q", rule.Name)
	}
	rule.ServiceName = serviceName.ToString()
	localPorts, err := oleutil.GetProperty(item, "LocalPorts")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (LocalPorts) of Rule %q", rule.Name)
	}
	rule.LocalPorts = localPorts.ToString()
	remotePorts, err := oleutil.GetProperty(item, "RemotePorts")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (RemotePorts) of Rule %q", rule.Name)
	}
	rule.RemotePorts = remotePorts.ToString()
	localAddresses, err := oleutil.GetProperty(item, "LocalAddresses")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (LocalAddresses) of Rule %q", rule.Name)
	}
	rule.LocalAddresses = localAddresses.ToString()
	remoteAddresses, err := oleutil.GetProperty(item, "RemoteAddresses")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (RemoteAddresses) of Rule %q", rule.Name)
	}
	rule.RemoteAddresses = remoteAddresses.ToString()
	icmpTypesAndCodes, err := oleutil.GetProperty(item, "ICMPTypesAndCodes")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (ICMPTypesAndCodes) of Rule %q", rule.Name)
	}
	rule.ICMPTypesAndCodes = icmpTypesAndCodes.ToString()
	grouping, err := oleutil.GetProperty(item, "Grouping")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Grouping) of Rule %q", rule.Name)
	}
	rule.Grouping = grouping.ToString()
	interfaceTypes, err := oleutil.GetProperty(item, "InterfaceTypes")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (InterfaceTypes) of Rule %q", rule.Name)
	}
	rule.InterfaceTypes = interfaceTypes.ToString()
	protocol, err := oleutil.GetProperty(item, "Protocol")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Protocol) of Rule %q", rule.Name)
	}
	rule.Protocol = protocol.Value().(int32)
	direction, err := oleutil.GetProperty(item, "Direction")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Direction) of Rule %q", rule.Name)
	}
	rule.Direction = direction.Value().(int32)
	action, err := oleutil.GetProperty(item, "Action")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Action) of Rule %q", rule.Name)
	}
	rule.Action = action.Value().(int32)
	enabled, err := oleutil.GetProperty(item, "Enabled")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Enabled) of Rule %q", rule.Name)
	}
	rule.Enabled = enabled.Value().(bool)
	edgeTraversal, err := oleutil.GetProperty(item, "EdgeTraversal")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (EdgeTraversal) of Rule %q", rule.Name)
	}
	rule.EdgeTraversal = edgeTraversal.Value().(bool)
	profiles, err := oleutil.GetProperty(item, "Profiles")
	if err != nil {
		return rule, fmt.Errorf("failed to get Property (Profiles) of Rule %q", rule.Name)
	}
	rule.Profiles = profiles.Value().(int32)

	return rule, nil
}

// FirewallGroupEnable allows to enable predefined firewall group. It is better
// to not use names as "File and Printer Sharing" because they are localized and
// your function will do not work on non-english Windows.
// Look at FILE_AND_PRINTER_SHARING const as example.
// This codes can be found here:
// https://windows10dll.nirsoft.net/firewallapi_dll.html
//
// You can enable group in selected FW profiles or in current,
// use something like:
//   NET_FW_PROFILE2_DOMAIN|NET_FW_PROFILE2_PRIVATE
// to enable group in given profiles.
func FirewallGroupEnable(name string, profile int32) error {
	return firewallGroup(name, profile, true)
}

// FirewallGroupDisable disables given group in given profiles. The same
// rules as for FirewallGroupEnable applies.
func FirewallGroupDisable(name string, profile int32) error {
	return firewallGroup(name, profile, false)
}

func firewallGroup(name string, profile int32, enable bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return err
	}
	defer firewallAPIRelease(u, fwPolicy)

	if profile == NET_FW_PROFILE2_CURRENT {
		currentProfiles, err := oleutil.GetProperty(fwPolicy, "CurrentProfileTypes")
		if err != nil {
			return fmt.Errorf("Failed to get CurrentProfiles: %s", err)
		}
		profile = currentProfiles.Value().(int32)
	}

	if _, err := oleutil.CallMethod(fwPolicy, "EnableRuleGroup", profile, name, enable); err != nil { //currentProfiles
		return fmt.Errorf("Error enabling group, %s", err)
	}

	return nil
}

// FirewallIsEnabled returns true if firewall is enabled for given profile.
// You can use all NET_FW_PROFILE2* constants but NET_FW_PROFILE2_ALL.
// It will return error if Firewall status can not be checked.
func FirewallIsEnabled(profile int32) (bool, error) {
	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return false, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	switch profile {
	case NET_FW_PROFILE2_CURRENT:
		currentProfiles, err := oleutil.GetProperty(fwPolicy, "CurrentProfileTypes")
		if err != nil {
			return false, fmt.Errorf("failed to get CurrentProfiles: %s", err)
		}
		profile = currentProfiles.Value().(int32)
	case NET_FW_PROFILE2_ALL:
		return false, fmt.Errorf("you can't use NET_FW_PROFILE2_ALL as parameter")
	}

	enabled, err := oleutil.GetProperty(fwPolicy, "FirewallEnabled", profile)

	return enabled.Value().(bool), err
}

// FirewallEnable enables firewall for given profile.
// If firewall is enabled already for profile it will return false.
func FirewallEnable(profile int32) (bool, error) {
	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return false, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	enabled, err := oleutil.GetProperty(fwPolicy, "FirewallEnabled", profile)
	if err != nil {
		return false, err
	}
	if enabled.Value().(bool) {
		return false, nil
	}

	_, err = oleutil.PutProperty(fwPolicy, "FirewallEnabled", profile, true)
	if err != nil {
		return false, err
	}
	return true, nil
}

// FirewallDisable disables firewall for given profile.
// If firewall is disabled already for profile it will return false.
func FirewallDisable(profile int32) (bool, error) {
	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return false, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	enabled, err := oleutil.GetProperty(fwPolicy, "FirewallEnabled", profile)
	if err != nil {
		return false, err
	}
	if !enabled.Value().(bool) {
		return false, nil
	}

	_, err = oleutil.PutProperty(fwPolicy, "FirewallEnabled", profile, false)
	if err != nil {
		return false, err
	}
	return true, nil
}

// FirewallCurrentProfiles return which profiles are currently active.
// Every active interface can have it's own profile. F.e.: Public for Wifi,
// Domain for VPN, and Private for LAN. All at the same time.
func FirewallCurrentProfiles() (FWProfiles, error) {
	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return FWProfiles{}, err
	}
	defer firewallAPIRelease(u, fwPolicy)
	currentProfiles, err := oleutil.GetProperty(fwPolicy, "CurrentProfileTypes")
	if err != nil {
		return FWProfiles{}, fmt.Errorf("failed to get FW CurrentProfiles: %s", err)
	}

	cp := firewallParseProfiles(currentProfiles.Value().(int32))

	if !(cp.Domain || cp.Private || cp.Public) {
		// is no active profile even possible? no network?
		return cp, fmt.Errorf("no active FW profile detected")
	}

	return cp, nil
}

// firewallParseProfiles returns FWProfiles struct which
// keeps which profiles are enabled for given integer.
func firewallParseProfiles(v int32) FWProfiles {
	var p FWProfiles
	if v&NET_FW_PROFILE2_DOMAIN != 0 {
		p.Domain = true
	}
	if v&NET_FW_PROFILE2_PRIVATE != 0 {
		p.Private = true
	}
	if v&NET_FW_PROFILE2_PUBLIC != 0 {
		p.Public = true
	}
	return p
}

// firewallRuleAdd is universal function to add all kinds of rules.
func firewallRuleAdd(name, description, group, appPath, serviceName, ports, remotePorts, localAddresses, remoteAddresses, icmpTypes string, protocol, direction, profile int32, enabled, edgeTraversal bool) (bool, error) {

	if name == "" {
		return false, fmt.Errorf("empty FW Rule name, name is mandatory")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u, fwPolicy, err := firewallAPIInit()
	if err != nil {
		return false, err
	}
	defer firewallAPIRelease(u, fwPolicy)

	if profile == NET_FW_PROFILE2_CURRENT {
		currentProfiles, err := oleutil.GetProperty(fwPolicy, "CurrentProfileTypes")
		if err != nil {
			return false, fmt.Errorf("Failed to get CurrentProfiles: %s", err)
		}
		profile = currentProfiles.Value().(int32)
	}
	unknownRules, err := oleutil.GetProperty(fwPolicy, "Rules")
	if err != nil {
		return false, fmt.Errorf("Failed to get Rules: %s", err)
	}
	rules := unknownRules.ToIDispatch()

	if ok, err := FirewallRuleExistsByName(rules, name); err != nil {
		return false, fmt.Errorf("Error while checking rules for duplicate: %s", err)
	} else if ok {
		return false, nil
	}

	unknown2, err := oleutil.CreateObject("HNetCfg.FWRule")
	if err != nil {
		return false, fmt.Errorf("Error creating Rule object: %s", err)
	}
	defer unknown2.Release()

	fwRule, err := unknown2.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return false, fmt.Errorf("Error creating Rule object (2): %s", err)
	}
	defer fwRule.Release()

	if _, err := oleutil.PutProperty(fwRule, "Name", name); err != nil {
		return false, fmt.Errorf("Error setting property (Name) of Rule: %s", err)
	}
	if _, err := oleutil.PutProperty(fwRule, "Description", description); err != nil {
		return false, fmt.Errorf("Error setting property (Description) of Rule: %s", err)
	}
	if appPath != "" {
		if _, err := oleutil.PutProperty(fwRule, "Applicationname", appPath); err != nil {
			return false, fmt.Errorf("Error setting property (Applicationname) of Rule: %s", err)
		}
	}
	if serviceName != "" {
		if _, err := oleutil.PutProperty(fwRule, "ServiceName", serviceName); err != nil {
			return false, fmt.Errorf("Error setting property (ServiceName) of Rule: %s", err)
		}
	}
	if protocol != 0 {
		if _, err := oleutil.PutProperty(fwRule, "Protocol", protocol); err != nil {
			return false, fmt.Errorf("Error setting property (Protocol) of Rule: %s", err)
		}
	}
	if icmpTypes != "" {
		if _, err := oleutil.PutProperty(fwRule, "IcmpTypesAndCodes", icmpTypes); err != nil {
			return false, fmt.Errorf("Error setting property (IcmpTypesAndCodes) of Rule: %s", err)
		}
	}
	if ports != "" {
		if _, err := oleutil.PutProperty(fwRule, "LocalPorts", ports); err != nil {
			return false, fmt.Errorf("Error setting property (LocalPorts) of Rule: %s", err)
		}
	}
	if remotePorts != "" {
		if _, err := oleutil.PutProperty(fwRule, "RemotePorts", remotePorts); err != nil {
			return false, fmt.Errorf("Error setting property (RemotePorts) of Rule: %s", err)
		}
	}
	if localAddresses != "" {
		if _, err := oleutil.PutProperty(fwRule, "LocalAddresses", localAddresses); err != nil {
			return false, fmt.Errorf("Error setting property (LocalAddresses) of Rule: %s", err)
		}
	}
	if remoteAddresses != "" {
		if _, err := oleutil.PutProperty(fwRule, "RemoteAddresses", remoteAddresses); err != nil {
			return false, fmt.Errorf("Error setting property (RemoteAddresses) of Rule: %s", err)
		}
	}
	if direction != 0 {
		if _, err := oleutil.PutProperty(fwRule, "Direction", direction); err != nil {
			return false, fmt.Errorf("Error setting property (Direction) of Rule: %s", err)
		}
	}
	if _, err := oleutil.PutProperty(fwRule, "Enabled", enabled); err != nil {
		return false, fmt.Errorf("Error setting property (Enabled) of Rule: %s", err)
	}
	if _, err := oleutil.PutProperty(fwRule, "Grouping", group); err != nil {
		return false, fmt.Errorf("Error setting property (Grouping) of Rule: %s", err)
	}
	if _, err := oleutil.PutProperty(fwRule, "Profiles", profile); err != nil {
		return false, fmt.Errorf("Error setting property (Profiles) of Rule: %s", err)
	}
	if _, err := oleutil.PutProperty(fwRule, "Action", NET_FW_ACTION_ALLOW); err != nil {
		return false, fmt.Errorf("Error setting property (Action) of Rule: %s", err)
	}
	if edgeTraversal {
		if _, err := oleutil.PutProperty(fwRule, "EdgeTraversal", edgeTraversal); err != nil {
			return false, fmt.Errorf("Error setting property (EdgeTraversal) of Rule: %s", err)
		}
	}

	if _, err := oleutil.CallMethod(rules, "Add", fwRule); err != nil {
		return false, fmt.Errorf("Error adding Rule: %s", err)
	}

	return true, nil
}

func FirewallRuleExistsByName(rules *ole.IDispatch, name string) (bool, error) {
	enumProperty, err := rules.GetProperty("_NewEnum")
	if err != nil {
		return false, fmt.Errorf("Failed to get enumeration property on Rules: %s", err)
	}
	defer enumProperty.Clear()

	enum, err := enumProperty.ToIUnknown().IEnumVARIANT(ole.IID_IEnumVariant)
	if err != nil {
		return false, fmt.Errorf("Failed to cast enum to correct type: %s", err)
	}
	if enum == nil {
		return false, fmt.Errorf("can't get IEnumVARIANT, enum is nil")
	}

	for itemRaw, length, err := enum.Next(1); length > 0; itemRaw, length, err = enum.Next(1) {
		if err != nil {
			return false, fmt.Errorf("Failed to seek next Rule item: %s", err)
		}

		t, err := func() (bool, error) {
			item := itemRaw.ToIDispatch()
			defer item.Release()

			if item, err := oleutil.GetProperty(item, "Name"); err != nil {
				return false, fmt.Errorf("Failed to get Property (Name) of Rule")
			} else if item.ToString() == name {
				return true, nil
			}

			return false, nil
		}()

		if err != nil {
			return false, err
		} else if t {
			return true, nil
		}
	}

	return false, nil
}

// firewallRulesEnum takes fwPolicy object and returns all objects which needs freeing and enum itself,
// which is used to enumerate rules. do not forget to:
//   defer firewallRulesEnumRealease(ur, ep)
func firewallRulesEnum(fwPolicy *ole.IDispatch) (*ole.VARIANT, *ole.VARIANT, *ole.IEnumVARIANT, error) {
	unknownRules, err := oleutil.GetProperty(fwPolicy, "Rules")
	if err != nil {
		return nil, unknownRules, nil, fmt.Errorf("failed to get Rules: %s", err)
	}
	rules := unknownRules.ToIDispatch()

	enumProperty, err := rules.GetProperty("_NewEnum")
	if err != nil {
		unknownRules.Clear()
		return nil, unknownRules, nil, fmt.Errorf("failed to get enumeration property on Rules: %s", err)
	}

	enum, err := enumProperty.ToIUnknown().IEnumVARIANT(ole.IID_IEnumVariant)
	if err != nil {
		enumProperty.Clear()
		unknownRules.Clear()
		return nil, unknownRules, nil, fmt.Errorf("failed to cast enum to correct type: %s", err)
	}
	if enum == nil {
		enumProperty.Clear()
		unknownRules.Clear()
		return nil, unknownRules, nil, fmt.Errorf("can't get IEnumVARIANT, enum is nil")
	}
	return unknownRules, enumProperty, enum, nil
}

// firewallRuleEnumRelease will free memory used by firewallRulesEnum.
func firewallRulesEnumRealease(unknownRules, enumProperty *ole.VARIANT) {
	enumProperty.Clear()
	unknownRules.Clear()
}

// firewallAPIInit initialize common fw api.
// then:
// dispatch firewallAPIRelease(u, fwp)
func firewallAPIInit() (*ole.IUnknown, *ole.IDispatch, error) {
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)

	unknown, err := oleutil.CreateObject("HNetCfg.FwPolicy2")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create FwPolicy Object: %s", err)
	}

	fwPolicy, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		unknown.Release()
		return nil, nil, fmt.Errorf("Failed to create FwPolicy Object (2): %s", err)
	}

	return unknown, fwPolicy, nil

}

// firewallAPIRelease cleans memory.
func firewallAPIRelease(u *ole.IUnknown, fwp *ole.IDispatch) {
	fwp.Release()
	u.Release()
	ole.CoUninitialize()
}
