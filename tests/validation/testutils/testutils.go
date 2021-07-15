package testutils

/*
type Cluster struct {
        nodes map[string]Node
        pods map[string]Pod
}
*/
import (
	"bytes"

	//"flag"
	"fmt"

	"golang.org/x/crypto/ssh"

	//"os"

	//"os"

	//"os"

	"log"

	"os/exec"

	"strings"
	"time"

	//"flag"
	"io/ioutil"
)

type Node struct {
	Name       string
	Status     string
	Roles      string
	InternalIP string
	ExternalIP string
}
type Pod struct {
	NameSpace string
	Name      string
	Ready     string
	Status    string
	Restarts  string
	NodeIP    string
	Node      string
}

var config *ssh.ClientConfig

var SSHKEY string
var SSHUSER string

func CheckError(e error) {
	if e != nil {
		panic(e)
	}
}

//var node_os = flag.String("node_os", "ubuntu", "a string")

var err error

func ConfigureSSH(host string, SSHUser string, SSHKey string) *ssh.Client {
	config = &ssh.ClientConfig{
		User: SSHUser,
		Auth: []ssh.AuthMethod{
			publicKey(SSHKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", host, config)
	fmt.Println(err)
	CheckError(err)
	return conn
}

func RunCmdOnNode(cmd string, ServerIP string, SSHUser string, SSHKey string) string {

	fmt.Println("DEPLOY")

	fmt.Println(SSHUser)
	fmt.Println(SSHKey)
	fmt.Println("RUnCmdOnNode method")
	fmt.Println(ServerIP)
	Server := ServerIP + ":22"
	fmt.Println(Server)

	conn := ConfigureSSH(Server, SSHUser, SSHKey)
	//conn := ConfigureSSH(Server)
	fmt.Println("RUnCmdOnNode method 1")
	fmt.Println(cmd)
	res := runsshCommand(cmd, conn)
	fmt.Println("RUnCmdOnNode method 2")
	res = strings.TrimSpace(res)
	fmt.Println("SDSFDSFASDFASFDSDF")
	fmt.Println(res)
	return res
}

func runsshCommand(cmd string, conn *ssh.Client) string {
	sess, err := conn.NewSession()
	if err != nil {
		panic(err)
	}
	defer sess.Close()
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	sess.Stdout = &stdoutBuf
	sess.Stderr = &stderrBuf

	sess.Run(cmd)
	//TODO Handle error here
	return fmt.Sprintf("%s", stdoutBuf.String())
}

func publicKey(path string) ssh.AuthMethod {
	key, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		panic(err)
	}
	return ssh.PublicKeys(signer)
}

func CountOfStringInSlice(str string, pods []Pod) int {
	count := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, str) {
			count++
		}
	}
	return count
}

func DeployWorkload(workload string, kubeconfig string) string {

	cmd := "kubectl apply -f " + workload + " --kubeconfig=" + kubeconfig
	return RunCommand(cmd)

}

func FetchClusterIP(kubeconfig string, servicename string) string {
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	fmt.Println(cmd)
	res := RunCommand(cmd)
	fmt.Println(res)
	return res
}

func FetchNodeExternalIp(kubeconfig string) []string {
	nodeExternalIPs := []string{"18.217.33.42", "13.58.67.125", "3.15.219.128"}
	/*
	   cmd := "kubectl get node --output=jsonpath='{range .items[*]} { .status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + kubeconfig
	   fmt.Println(cmd)
	   res := RunCommand(cmd)
	   fmt.Println("External IPs")
	   fmt.Println(res)
	   node_external_ip := strings.Trim(res, " ")
	   nodeExternalIPs := strings.Split(node_external_ip, " ")
	*/
	fmt.Println(nodeExternalIPs)
	return nodeExternalIPs
}

func FetchNodeInternalIp(kubeconfig string) string {
	cmd := "kubectl get node --output=jsonpath='{range .items[*]} { .status.addresses[?(@.type==\"InternalIP\")].address}' --kubeconfig=" + kubeconfig
	fmt.Println(cmd)
	res := RunCommand(cmd)
	return res
}

func FetchpodIP(kubeconfig string) string {
	cmd := "kubectl get pods -l name=testnginx -o yaml |tr -s ' ' |grep  -w \"^ podIP\" || cut -d \" \" -f3" + kubeconfig
	res := RunCommand(cmd)
	return res
}

func FetchPodName(kubeconfig string, selector string) []string {
	cmd := "kubectl get pods --selector " + selector + " --no-headers --kubeconfig=" + kubeconfig + " |awk '{print $1}'"
	fmt.Println(cmd)
	res := RunCommand(cmd)
	fmt.Println(res)
	pods := strings.TrimSpace(res)
	podnames := strings.Split(pods, "\n")

	return podnames
}
func ParseNode(kubeconfig string) []Node {
	nodes := make([]Node, 0, 10)
	var node Node

	cmd := "kubectl get nodes --no-headers -o wide -A --kubeconfig=" + kubeconfig

	//fmt.Println(cmd)
	res := RunCommand(cmd)
	res = strings.TrimSpace(res)

	fmt.Println(res)
	split := strings.Split(res, "\n")

	for _, rec := range split {
		fields := strings.Fields(string(rec))
		node.Name = fields[0]
		node.Status = fields[1]
		node.Roles = fields[2]
		node.InternalIP = fields[5]
		node.ExternalIP = fields[6]
		nodes = append(nodes, node)
	}
	return nodes
}

func ParsePod(kubeconfig string) []Pod {
	pods := make([]Pod, 0, 10)
	var pod Pod
	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + kubeconfig

	fmt.Println(cmd)

	res := RunCommand(cmd)
	res = strings.TrimSpace(res)
	fmt.Println(res)

	split := strings.Split(res, "\n")

	for _, rec := range split {
		fields := strings.Fields(string(rec))
		pod.NameSpace = fields[0]
		pod.Name = fields[1]
		pod.Ready = fields[2]
		pod.Status = fields[3]
		pod.Restarts = fields[4]
		pod.NodeIP = fields[6]
		pod.Node = fields[7]
		pods = append(pods, pod)
	}
	return pods
}

func RunCommand(cmd string) string {
	c := exec.Command("bash", "-c", cmd)
	//time.Sleep(20 * time.Second)

	var out bytes.Buffer
	c.Stdout = &out
	err := c.Run()
	if err != nil {
		log.Fatal(err)
	}
	return out.String()
}

func InstallWithFlag(install_type string, flag string, hostip string, in_config bool, SSHUser string, SSHKey string) {
	//("yum_install", "--write-kubeconfig-mode 644", master_ips[0], "true")
	fmt.Println(flag)
	cmd := "sudo mkdir -p /etc/rancher/rke2"
	_ = RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)
	fmt.Println("DIR")
	cmd = "sudo touch /etc/rancher/rke2/flags.conf"
	_ = RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)
	fmt.Println("FILE")
	time.Sleep(10 * time.Second)
	cmd = "sudo chmod 666 /etc/rancher/rke2/flags.conf"
	_ = RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)
	//time.Sleep(10 * time.Second)

	if in_config {
		fmt.Println("SSSSSSSS")
		//open the flags.conf, put the flag, call RunCmdOnNode
		write_to_file("\"/etc/rancher/rke2/flags.conf\"", flag, hostip, SSHUser, SSHKey)
		//assert file is created
		if install_type == "yum_install" {
			cmd := "sudo yum -y install rke2-server"
			res := RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)
			fmt.Println(res)
			cmd = "sudo systemctl restart rke2-server" //Check start restart and if cmd is common for all installs
			res = RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)
			time.Sleep(30 * time.Second)
			fmt.Println("SSSFFFGGG")
		} else {
			//if install type is not yum_install add cmd here
		}
	} else {

		cmd := "install cmd with flag" //install_type is not yum anyway
		res := RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)
		fmt.Println(res)
		//cmd = "systemctl restart rke2-user" //Check start restart and if cmd is common for all i
		//res = RunCmdOnNode(cmd, hostip)
	}

}
func write_to_file(filename string, flag string, hostip string, SSHUser string, SSHKey string) {
	fmt.Println("Config file")
	fmt.Println(filename)
	fmt.Println(hostip)
	// cmd := "f, err := os.Create(" + filename + ")"
	//cmd :="f, err := os.Create(\"/etc/rancher/rke2/flags.conf\")"
	cmd := "sudo " + flag + " > /etc/rancher/rke2/flags.conf"
	fmt.Println(cmd)

	res := RunCmdOnNode(cmd, hostip, SSHUser, SSHKey)

	fmt.Println(flag)
	fmt.Println(res)
	//cmd = "ioutil.WriteFile(" + filename + ", []byte(" + flag + "), 0666)"
	//fmt.Println(cmd)
	//err = RunCmdOnNode(cmd, hostip)

	//fmt.Println(err)
	/*
	   if err != "" {
	           fmt.Println("ERROR")
	           fmt.Println(err)
	           panic(err)
	   }
	*/

}

func VerifyNetworking(nodeexternalip string, port string, kubeconfig string) string {
	cmd := "kubectl run nginx  --replicas=2 --labels='app=nginx' --image=nginx --port=80 --kubeconfig=" + kubeconfig
	_ = RunCommand(cmd)
	cmd = "curl -L --insecure http://" + nodeexternalip + ":" + port + " | grep title"
	fmt.Println(cmd)
	res := RunCommand(cmd)

	fmt.Println(res)
	return res
}

func VerifyNetwork(ip string, port string, kubeconfig string, endpoint string) string {
	cmd := "curl -L --insecure http://" + ip + ":" + port + endpoint
	fmt.Println(cmd)
	res := RunCommand(cmd)
	fmt.Println(res)
	return res

}

/*
func VerifyNetworking(node_external_ip, port) {
        cmd := "curl -L --insecure --header https://" + node_external_ip + ":30803"
        res := RunCommand(cmd)
        return res
}

func Va(p_client, cluster, workloads, host, path,
insecure_redirect=False):
time.sleep(10)
curl_args = " "
if (insecure_redirect):
curl_args = " -L --insecure "
if len(host) > 0:
curl_args += " --header 'Host: " + host + "'"
nodes = get_schedulable_nodes(cluster, os_type="linux")
target_name_list = get_target_names(p_client, workloads)
for node in nodes:
host_ip = resolve_node_ip(node)
url = "http://" + host_ip + path
if not insecure_redirect:
wait_until_ok(url, timeout=300, headers={
"Host": host
})
cmd = curl_args + " " + url
validate_http_response(cmd, target_name_list)


func ValidateCustomPod(podList []string) (podStatus string, podCount int){
        for _, podName := range podList {
                for {
                        line, _, _ := buf.ReadLine()
                        if line == nil {
                                break
                        }
                        fmt.Println(string(line))

                        if strings.HasPrefix(string(line), podName) {
                                return podName
                        }
                }
        }
        podName = ""
        return
}


func UpgradeCluster(upgrade_k3s_version, node_ips, t *testing.T) {
        cmd := "curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=" + upgrade_k3s_version + " sh -s - " + flag // k3s version?
        for _, ip := range node_ips {
            RunCmdOnNode(cmd, ip)
        }
}



func RunCommand(cmd string, conn *ssh.Client) string {
        sess, err := conn.NewSession()
        if err != nil {
                panic(err)
        }
        defer sess.Close()
        var stdoutBuf bytes.Buffer
        var stderrBuf bytes.Buffer
        sess.Stdout = &stdoutBuf
        sess.Stderr = &stderrBuf

        sess.Run(cmd)
        //TODO Handle error here
        return fmt.Sprintf("%s", stdoutBuf.String())
}



*/
