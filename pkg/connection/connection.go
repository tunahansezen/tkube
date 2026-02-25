package connection

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	enc "com.github.tunahansezen/tkube/pkg/encryption"
	"com.github.tunahansezen/tkube/pkg/util"
	"github.com/guumaster/logsymbols"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

var (
	homedir        string
	sshDataFile    = fmt.Sprintf("%s/ssh", util.LocalDataPath)
	sshConnections = make(map[string]*ssh.Client)
	Nodes          = make(map[string]*Node)
)

type Node struct {
	IP                net.IP
	SSHUser           string
	SSHPass           string
	SSHPrivateKeyPath string
}

func (n Node) String() string {
	return n.SSHUser + "@" + n.IP.String()
}

func init() {
	homedir, _ = os.UserHomeDir()
	sshDataFile = strings.ReplaceAll(sshDataFile, "$HOME", homedir)
}

func IsReachable(host string, port int) bool {
	log.Debugf("Checking \"%s:%d\" is reachable", host, port)
	returnBool := false
	timeout := 5 * time.Second
	conn, _ := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if conn != nil {
		defer func() {
			err := conn.Close()
			if err != nil {
				log.Errorf("Error occurred while closing connection with %s:%d", host, port)
			}
		}()
		returnBool = true
	}
	if returnBool {
		log.Debugf("%s:%d is reachable", host, port)
	}
	return returnBool
}

func IsReachableURL(url string) bool {
	log.Debugf("Checking \"%s\" url is reachable", url)
	var host string
	var port int
	if strings.HasPrefix(url, "http") {
		urlBase := strings.Split(url, "://")[1]
		if strings.Contains(urlBase, ":") {
			s1 := strings.Split(strings.Split(urlBase, "/")[0], ":")
			host = s1[0]
			port, _ = strconv.Atoi(s1[1])
		} else {
			host = strings.Split(urlBase, "/")[0]
			if strings.HasPrefix(url, "https") {
				port = 443
			} else {
				port = 80
			}
		}
	} else {
		s1 := strings.Split(url, ":")
		host = s1[0]
		if len(s1) == 1 {
			port = 443
		} else {
			port, _ = strconv.Atoi(s1[1])
		}
	}
	return IsReachable(host, port)
}

func CheckSSHConnection(node *Node) error {
	if sshConnections[node.IP.String()] != nil {
		// already checked
		return nil
	}
	if !IsReachable(node.IP.String(), 22) {
		return errors.New(fmt.Sprintf("%s:%d is not reachable", node.IP, 22))
	}
	_, err := CreateSshConnection(node)
	return err
}

func CreateSshConnection(node *Node) (*ssh.Client, error) {
	exist := sshConnections[node.IP.String()]
	if exist != nil {
		return exist, nil
	}
	util.StartSpinner(fmt.Sprintf("Checking SSH connection to %s", node))
	var auth []ssh.AuthMethod
	var connection *ssh.Client
	var err error
	var usedSSHUser string
	var usedSSHPass string
	var usedPrivateKeyPath string
	var finalMsg string
	dataSSHUser, dataSSHPass, dataSSHPrivateKey, err := CheckSSHDataForAddr(node.IP.String())
	if err != nil {
		return nil, err
	}
	if dataSSHUser != "" && (dataSSHPass != "" || dataSSHPrivateKey != "") { // read from saved data
		usedSSHUser = dataSSHUser
		if dataSSHPass != "" {
			auth = []ssh.AuthMethod{
				ssh.Password(dataSSHPass),
			}
			usedSSHPass = dataSSHPass
		} else {
			auth = getKeyAuth(dataSSHPrivateKey)
			usedPrivateKeyPath = dataSSHPrivateKey
		}
		connection, err = sshDial(dataSSHUser, auth, node.IP.String())
	} else if node.SSHUser != "" && (node.SSHPass != "" || node.SSHPrivateKeyPath != "") { // read from config
		usedSSHUser = node.SSHUser
		if node.SSHPass != "" {
			auth = []ssh.AuthMethod{
				ssh.Password(node.SSHPass),
			}
			usedSSHPass = node.SSHPass
		} else {
			auth = getKeyAuth(node.SSHPrivateKeyPath)
			usedPrivateKeyPath = node.SSHPrivateKeyPath
		}
		connection, err = sshDial(node.SSHUser, auth, node.IP.String())
	} else { // ask credentials
		usedSSHUser, err = util.AskString(fmt.Sprintf("Please enter SSH user for %s", node.IP), false,
			util.CommonValidator)
		if err != nil {
			return nil, err
		}
		var authMethod string
		authMethod, err = util.AskChoice("Which method want to use for SSH authentication?",
			[]string{"password", "private-key"})
		if err != nil {
			return nil, err
		} else if authMethod == "password" {
			usedSSHPass, err = util.AskString(fmt.Sprintf("Please enter SSH pass for %s", node.IP), true,
				util.PasswordValidator)
			if err != nil {
				return nil, err
			}
			auth = []ssh.AuthMethod{
				ssh.Password(usedSSHPass),
			}
		} else if authMethod == "private-key" {
			usedPrivateKeyPath, err = util.AskString(
				fmt.Sprintf("Please enter SSH private key path for %s", node.IP), false, util.PathValidator)
			if err != nil {
				return nil, err
			}
			auth = getKeyAuth(usedPrivateKeyPath)
		}
		connection, err = sshDial(usedSSHUser, auth, node.IP.String())
		if err == nil {
			finalMsg = fmt.Sprintf("SSH connection successful for %s with user \"%s\"", node.IP, usedSSHUser)
		}
	}

	if err != nil {
		util.StopSpinner(fmt.Sprintf("SSH authentication failed for %s", node.IP), logsymbols.Error)
		if dataSSHUser != "" {
			err = clearSSHDataForAddr(node.IP.String())
			if err != nil {
				return nil, err
			}
		}
		return nil, err
	}
	sshConnections[node.IP.String()] = connection
	if dataSSHUser == "" && (usedSSHUser != "" || usedSSHPass != "") {
		err = WriteSSHData(node.IP.String(), usedSSHUser, usedSSHPass, usedPrivateKeyPath)
		if err != nil {
			return nil, err
		}
	}
	util.StopSpinner(finalMsg, logsymbols.Success)
	node.SSHUser = usedSSHUser
	node.SSHPass = usedSSHPass
	Nodes[node.IP.String()] = node
	return connection, nil
}

func WriteSSHData(addr string, sshUser string, sshPass string, sshPrivateKeyPath string) error {
	err := touchFile(sshDataFile)
	if err != nil {
		log.Debugf("Error occurred while creating connection data file: %s", sshDataFile)
		return err
	}
	file, _ := os.ReadFile(sshDataFile)
	file, err = enc.DecryptFile(file)
	if err != nil {
		log.Debugf("Error occurred while decrypting connection data file: %s", sshDataFile)
		return err
	}
	data := make(map[string]interface{})
	err = yaml.Unmarshal(file, &data)
	if err != nil {
		log.Debugf("Error occurred while parsing connection data file: %s", sshDataFile)
		return err
	}
	d2 := make(map[string]string)
	d2["sshUser"] = sshUser
	d2["sshPass"] = sshPass
	d2["sshPrivateKeyPath"] = sshPrivateKeyPath
	data[addr] = d2
	out, _ := yaml.Marshal(data)
	out, err = enc.EncryptFile(out)
	if err != nil {
		log.Debugf("Error occurred while encrypting connection data file: %s", sshDataFile)
		return err
	}
	err = os.WriteFile(sshDataFile, out, os.FileMode(0666))
	if err != nil {
		log.Debugf("Error occurred while writing to connection data file: %s", sshDataFile)
		return err
	}
	return nil
}

func touchFile(name string) error {
	folder := name[:strings.LastIndexAny(name, "/")]
	cmd := exec.Command("bash", "-c", fmt.Sprintf("mkdir -p %s", folder))
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(name, os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	return file.Close()
}

func CheckSSHDataForAddr(addr string) (sshUser, sshPass, sshPrivateKeyPath string, err error) {
	file, _ := os.ReadFile(sshDataFile)
	file, err = enc.DecryptFile(file)
	if err != nil {
		log.Debugf("Error occurred while decrypting connection data file: %s", sshDataFile)
		return "", "", "", err
	}
	data := make(map[string]map[string]string)
	err = yaml.Unmarshal(file, &data)
	if err != nil {
		log.Debugf("Error occurred while parsing connection data file: %s", sshDataFile)
		return "", "", "", err
	}
	sshUser = data[addr]["sshUser"]
	sshPass = data[addr]["sshPass"]
	sshPrivateKeyPath = data[addr]["sshPrivateKeyPath"]
	return sshUser, sshPass, sshPrivateKeyPath, nil
}

func clearSSHDataForAddr(addr string) error {
	file, _ := os.ReadFile(sshDataFile)
	file, err := enc.DecryptFile(file)
	if err != nil {
		log.Debugf("Error occurred while decrypting connection data file: %s", sshDataFile)
		return err
	}
	data := make(map[string]map[string]string)
	err = yaml.Unmarshal(file, &data)
	if err != nil {
		log.Debugf("Error occurred while parsing connection data file: %s", sshDataFile)
		return err
	}
	delete(data, addr)
	out, _ := yaml.Marshal(data)
	out, err = enc.EncryptFile(out)
	if err != nil {
		log.Debugf("Error occurred while encrypting connection data file: %s", sshDataFile)
		return err
	}
	err = os.WriteFile(sshDataFile, out, os.FileMode(0666))
	if err != nil {
		log.Debugf("Error occurred while writing to connection data file: %s", sshDataFile)
		return err
	}
	return nil
}

func getKeyAuth(keyPath string) []ssh.AuthMethod {
	pemBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil
	}
	// create signer
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil
	}
	return []ssh.AuthMethod{
		ssh.PublicKeys(signer),
	}
}

func sshDial(user string, auth []ssh.AuthMethod, addr string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lgtm[go/insecure-hostkeycallback]
		Auth:            auth,
	}
	return ssh.Dial("tcp", net.JoinHostPort(addr, "22"), config)
}

func CloseSSHSessions() {
	for addr, connection := range sshConnections {
		err := connection.Close()
		if err != nil {
			log.Errorf("Error occurred while closing SSH connection with %s", addr)
			continue
		}
		log.Debugf("Connection closed: %s", addr)
	}
}

func SendFile(ip net.IP, srcFile io.Reader, dstPath string) error {
	exist := sshConnections[ip.String()]
	if exist == nil {
		client, err := CreateSshConnection(&Node{IP: ip})
		if err != nil {
			return err
		}
		exist = client
	}
	sftpClient, err := sftp.NewClient(exist)
	if err != nil {
		return err
	}
	defer func() {
		err = sftpClient.Close()
		if err != nil {
			log.Errorf("Error occurred while closing sftp client with %s", ip)
		}
	}()

	// Create the destination file
	dstFile, err := sftpClient.Create(dstPath)
	if err != nil {
		return err
	}
	defer func() {
		err = dstFile.Close()
		if err != nil {
			log.Errorf("Error occurred while closing file \"%s\" on %s", dstPath, ip)
		}
	}()

	// write to file
	if _, err = dstFile.ReadFrom(srcFile); err != nil {
		return err
	}
	return nil
}
