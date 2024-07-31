//go:build windows
// +build windows

package windows

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Microsoft/hcsshim"
	wapi "github.com/iamacarpet/go-win64api"
	"github.com/libp2p/go-netroute"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	opv1 "github.com/tigera/operator/api/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	replaceSlashWin = template.FuncMap{
		"replace": func(s string) string {
			return strings.ReplaceAll(s, "\\", "\\\\")
		},
	}
)

// createHnsNetwork creates the network that will connect nodes and returns its managementIP
func createHnsNetwork(backend string, networkAdapter string) (string, error) {
	var network hcsshim.HNSNetwork
        // Check if the interface already exists
	hcsnetwork, err := hcsshim.GetHNSNetworkByName(CalicoHnsNetworkName)
	if err == nil {
		return hcsnetwork.ManagementIP, nil
	}

	if backend == "vxlan" {
		// Ignoring the return because both true and false without an error represent that the firewall rule was created or already exists
		if _, err := wapi.FirewallRuleAdd("OverlayTraffic4789UDP", "Overlay network traffic UDP", "", "4789", wapi.NET_FW_IP_PROTOCOL_UDP, wapi.NET_FW_PROFILE2_ALL); err != nil {
			return "", fmt.Errorf("error creating firewall rules: %v", err)
		}
		logrus.Infof("Creating VXLAN network using the vxlanAdapter: %s", networkAdapter)
		network = hcsshim.HNSNetwork{
			Type:               "Overlay",
			Name:               CalicoHnsNetworkName,
			NetworkAdapterName: networkAdapter,
			Subnets: []hcsshim.Subnet{
				{
					AddressPrefix:  "192.168.255.0/30",
					GatewayAddress: "192.168.255.1",
					Policies: []json.RawMessage{
						[]byte("{ \"Type\": \"VSID\", \"VSID\": 9999 }"),
					},
				},
			},
		}
	} else {
		network = hcsshim.HNSNetwork{
			Type:               "L2Bridge",
			Name:               CalicoHnsNetworkName,
			NetworkAdapterName: networkAdapter,
			Subnets: []hcsshim.Subnet{
				{
					AddressPrefix:  "192.168.255.0/30",
					GatewayAddress: "192.168.255.1",
				},
			},
		}
	}

	if _, err := network.Create(); err != nil {
		return "", fmt.Errorf("error creating the %s network: %v", CalicoHnsNetworkName, err)
	}

	// Check if network exists. If it does not after 5 minutes, fail
	for start := time.Now(); time.Since(start) < 5*time.Minute; {
		network, err := hcsshim.GetHNSNetworkByName(CalicoHnsNetworkName)
		if err == nil {
			return network.ManagementIP, nil
		}
	}

	return "", fmt.Errorf("failed to create %s network", CalicoHnsNetworkName)
}

// nodeAddressAutodetection processes the HelmChartConfig info and returns the nodeAddressAutodetection method expected by Calico config
func nodeAddressAutodetection(autoDetect opv1.NodeAddressAutodetection) (string, error) {
	if autoDetect.FirstFound != nil && *autoDetect.FirstFound {
		return "first-found", nil
	}

	if autoDetect.CanReach != "" {
		return "can-reach=" + autoDetect.CanReach, nil
	}

	if autoDetect.Interface != "" {
		return "interface=" + autoDetect.Interface, nil
	}

	if len(autoDetect.CIDRS) > 0 {
		return "cidrs=" + strings.Join(autoDetect.CIDRS, ","), nil
	}

	return "", errors.New("the passed autoDetect value is not supported")
}

// deleteAllNetworks deletes all hns networks
func deleteAllNetworks() error {
	networks, err := hcsshim.HNSListNetworkRequest("GET", "", "")
	if err != nil {
		return err
	}

	var ips []string

	for _, network := range networks {
		if network.Name != "nat" {
			logrus.Debugf("Deleting network: %s before starting calico", network.Name)
			ips = append(ips, network.ManagementIP)
			_, err = network.Delete()
			if err != nil {
				return err
			}
		}
	}

	// HNS overlay networks restart the physical interface when they are deleted. Wait until it comes back before returning
	// TODO: Replace with non-deprecated PollUntilContextTimeout when our and Kubernetes code migrate to it
	waitErr := wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
		for _, ip := range ips {
			logrus.Debugf("Calico is waiting for the interface with ip: %s to come back", ip)
			_, err := findInterface(ip)
			if err != nil {
				return false, nil
			}
		}
		return true, nil
	})

	if waitErr == wait.ErrWaitTimeout {
		return fmt.Errorf("timed out waiting for the network interfaces to come back")
	}

	return nil
}

// platformType returns the platform where we are running
func platformType() (string, error) {
	aksNet, _ := hcsshim.GetHNSNetworkByName("azure")
	if aksNet != nil {
		return "aks", nil
	}

	eksNet, _ := hcsshim.GetHNSNetworkByName("vpcbr*")
	if eksNet != nil {
		return "eks", nil
	}

	// EC2
	ec2Resp, err := http.Get("http://169.254.169.254/latest/meta-data/local-hostname")
	if err != nil && hasTimedOut(err) {
		return "", err
	}
	if ec2Resp != nil {
		defer ec2Resp.Body.Close()
		if ec2Resp.StatusCode == http.StatusOK {
			return "ec2", nil
		}
	}

	// GCE
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/hostname", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	gceResp, err := client.Do(req)
	if err != nil && hasTimedOut(err) {
		return "", err
	}
	if gceResp != nil {
		defer gceResp.Body.Close()
		if gceResp.StatusCode == http.StatusOK {
			return "gce", nil
		}
	}

	return "bare-metal", nil
}

func hasTimedOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *net.OpError:
		if err.Timeout() {
			return true
		}
	}
	errTxt := "use of closed network connection"
	if err != nil && strings.Contains(err.Error(), errTxt) {
		return true
	}
	return false
}

func autoConfigureIpam(it string) bool {
	if it == "host-local" {
		return true
	}
	return false
}

// setMetaDataServerRoute returns the metadata server for gce and ec2
func setMetaDataServerRoute(mgmt string) error {
	ip := net.ParseIP(mgmt)
	if ip == nil {
		return fmt.Errorf("not a valid ip")
	}

	metaIp := net.ParseIP("169.254.169.254/32")
	router, err := netroute.New()
	if err != nil {
		return err
	}

	_, _, preferredSrc, err := router.Route(ip)
	if err != nil {
		return err
	}

	_, _, _, err = router.RouteWithSrc(nil, preferredSrc, metaIp) // input not used on windows
	return err
}

// findInterfaceRegEx finds the interface that matches the regex
func findInterfaceRegEx(expression string) (string, error) {
	// We remove any whitespace character
	ifRegexes := regexp.MustCompile(`\s*,\s*`).Split(expression, -1)

	// Prepare the regex expression (e.g. (eth.?)|( wlan0)|( docker*))
	includeRegexes, err := regexp.Compile("(" + strings.Join(ifRegexes, ")|(") + ")")

	iFaces, err := net.Interfaces()
	if err != nil {
		fmt.Println(err)
	}
	for _, iFace := range iFaces {
		include := includeRegexes.MatchString(iFace.Name)
		if include {
			return iFace.Name, nil
		}
	}

	return "", fmt.Errorf("no interface matches the set expression")
}

// findInterfaceReach finds the interface used to reach the dest
func findInterfaceReach(dest string) (string, error) {
	destUdpAddress := fmt.Sprintf("[%s]:80", dest)
	conn, err := net.Dial("udp4", destUdpAddress)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddress := conn.LocalAddr()
	if localAddress == nil {
		return "", fmt.Errorf("no interface can route IP: %s", dest)
	}

	foundIf, err := findInterface(strings.Split(localAddress.String(), ":")[0])
	if err != nil {
		return "", err
	}

	return foundIf, nil
}

// findInterfaceCIDR returns the name of the interface whose IP is in the passed CIDR
func findInterfaceCIDR(cidrs []string) (string, error) {
	var foundIf string

	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return "", err
		}
		interfaces, err := net.Interfaces()
		if err != nil {
			return "", err
		}

		for _, i := range interfaces {
			addrs, err := i.Addrs()
			if err != nil {
				return "", err
			}
			for _, addr := range addrs {
				address := strings.Split(addr.String(), "/")[0]
				if network.Contains(net.ParseIP(address)) {
					if (foundIf != "") && (foundIf != i.Name) {
						return "", fmt.Errorf("the passed CIDRs are routed in different interfaces")
					}
					foundIf = i.Name
				}
			}
		}
	}

	if foundIf == "" {
		logrus.Errorf("no interface routes cidrs: %v", cidrs)
	}

	return foundIf, nil
}

// findInterface returns the name of the interface that contains the passed ip
func findInterface(ip string) (string, error) {
	iFaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iFace := range iFaces {
		addrs, err := iFace.Addrs()
		if err != nil {
			return "", err
		}
		logrus.Debugf("evaluating if the interface: %s with addresses %v, contains ip: %s", iFace.Name, addrs, ip)
		for _, addr := range addrs {
			if strings.Contains(addr.String(), ip) {
				return iFace.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no interface has the ip: %s", ip)
}
