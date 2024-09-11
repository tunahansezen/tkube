package os

import (
	"bytes"
	conn "com.github.tunahansezen/tkube/pkg/connection"
	"com.github.tunahansezen/tkube/pkg/constant"
	"com.github.tunahansezen/tkube/pkg/util"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/guumaster/logsymbols"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	OS                   Type
	InstallerType        Installer
	RemoteNodeIP         net.IP
	RemoteNode           *conn.Node
	sudoersPrevExistsMap = make(map[string]bool) // ip: prevExist
)

type Type int

const (
	Ubuntu Type = iota
	CentOS
	Rocky
)

type Installer int

const (
	Apt Installer = iota
	Yum
	Dnf
)

func DetectOS() {
	osOutput := RunCommand("awk -F= '/^NAME/{print $2}' /etc/os-release | tr -d '\"'", true)
	osOutputLower := strings.ToLower(osOutput)
	if strings.Contains(osOutputLower, "ubuntu") {
		OS = Ubuntu
		InstallerType = Apt
	} else if strings.Contains(osOutputLower, "centos") {
		OS = CentOS
		InstallerType = Yum
	} else if strings.Contains(osOutputLower, "rocky") {
		OS = Rocky
		InstallerType = Dnf
	} else {
		Exit(fmt.Sprintf("Unsupported OS: %s", osOutput), 1)
	}
}

func RunCommand(command string, silent bool) string {
	return RunCommandOn(command, RemoteNode.IP, silent)
}

func RunCommandReturnError(command string, silent bool) (string, error) {
	return runCommandOnReturnErr(command, RemoteNode.IP, silent, true)
}

func runCommandOnReturnErr(command string, ip net.IP, silent, returnErr bool) (string, error) {
	var returnStr string
	var err error
	log.Tracef("CMD - ip: \"%s\" - command: \"%s\"", ip, command)
	if ip == nil {
		returnStr, err = localRun(command, silent)
	} else {
		node := conn.Nodes[ip.String()]
		if node == nil {
			node = &conn.Node{IP: ip}
		}
		returnStr, err = RemoteRun(node, command, silent)
	}
	if err != nil && !returnErr {
		log.Debugf("ip: \"%s\" - command: \"%s\"", ip, command)
		Exit(err.Error(), 1)
	}
	log.Tracef("RETURNSTR - \"%s\"", returnStr)
	return returnStr, err
}

func RunCommandOn(command string, ip net.IP, silent bool) string {
	output, _ := runCommandOnReturnErr(command, ip, silent, false)
	return output
}

func localRun(command string, silent bool) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	if !silent {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	out, err := cmd.CombinedOutput()
	var returnStr string
	var returnErr error
	if err != nil {
		returnStr, returnErr = strings.TrimSuffix(string(out), "\n"), err
		returnStr, returnErr = strings.TrimSuffix(returnStr, "\nlogout"), err
		log.Debug(returnStr)
	} else {
		returnStr, returnErr = strings.TrimSuffix(string(out), "\n"), nil
		returnStr, returnErr = strings.TrimSuffix(returnStr, "\nlogout"), nil
	}
	return returnStr, returnErr
}

func RemoteRun(node *conn.Node, cmd string, silent bool) (string, error) {
	connection, err := conn.CreateSshConnection(node)
	if err != nil {
		return "", err
	}

	// create a session.
	session, _ := connection.NewSession()
	var bs bytes.Buffer
	session.Stdout = &bs
	var be bytes.Buffer
	session.Stderr = &be
	if !silent {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
	}

	err = session.Run(cmd)
	if (InstallerType == Yum || InstallerType == Dnf) && err != nil &&
		strings.Contains(cmd, "check-update") && err.(*ssh.ExitError).ExitStatus() == 100 {
		err = nil
	}
	var returnStr string
	var returnErr error
	if err != nil {
		errStr := strings.TrimSuffix(be.String(), "\n")
		returnStr, returnErr = errStr, errors.New(errStr)
		log.Debug(returnStr)
	} else {
		returnStr, returnErr = strings.TrimSuffix(bs.String(), "\n"), nil
	}
	return returnStr, returnErr
}

func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func AddToSudoers(ip net.IP) {
	node := conn.Nodes[ip.String()]
	returnStr := RunCommandOn(fmt.Sprintf("echo \"%s\" | sudo -S grep -qxF '%s ALL=NOPASSWD: ALL' /etc/sudoers && "+
		"echo exist || echo not exist", node.SSHPass, node.SSHUser), ip, true)
	if returnStr != "exist" {
		sudoersPrevExistsMap[ip.String()] = false
		RunCommandOn(fmt.Sprintf("echo \"%s\" | sudo -S sh -c \"echo '%s ALL=NOPASSWD: ALL' >> /etc/sudoers\"",
			node.SSHPass, node.SSHUser), ip, true)
		log.Debugf("\"%s\" user added to sudoers", node.SSHUser)
	} else {
		sudoersPrevExistsMap[ip.String()] = true
	}
}

func RemoveFromSudoers(ip net.IP) {
	node := conn.Nodes[ip.String()]
	RunCommandOn(fmt.Sprintf("sudo awk '!/%s ALL=NOPASSWD: ALL/' /etc/sudoers > temp && "+
		"sudo chown root:root temp && sudo mv temp /etc/sudoers",
		node.SSHUser), ip, true)
	log.Debugf("\"%s\" user removed from sudoers", node.SSHUser)
}

func AppendLineOn(line, file string, ifNotExists bool, ip net.IP) {
	if ifNotExists {
		RunCommandOn(fmt.Sprintf("grep -qxF '%s' %s || echo -e '%s' | sudo tee -a %s", line, file, line, file),
			ip, true)
	} else {
		RunCommandOn(fmt.Sprintf("echo  -e'%s' | sudo tee -a %s", line, file), ip, true)
	}
}

func AddGpgKey(url string, name string, ip net.IP) (path string) {
	if url == "" {
		log.Debugf("No key found for %s repo.", name)
		return ""
	}
	var dir string
	if InstallerType == Apt {
		dir = "/etc/apt/keyrings"
	} else if InstallerType == Yum || InstallerType == Dnf {
		dir = "/etc/pki/rpm-gpg"
	}
	RunCommandOn(fmt.Sprintf("sudo mkdir -p %s", dir), ip, true)
	path = fmt.Sprintf("%s/%s.gpg", dir, name)
	if InstallerType == Apt {
		RunCommandOn(fmt.Sprintf("curl -fsSL %s | sudo gpg --dearmor --yes -o %s", url, path), ip, true)
	} else if InstallerType == Yum || InstallerType == Dnf {
		RunCommandOn(fmt.Sprintf("curl -fsSL %s | sudo tee %s >/dev/null", url, path), ip, true)
		RunCommandOn(fmt.Sprintf("sudo rpm --import %s", path), ip, true)
	}
	return path
}

func AddRepository(name, shortname, repoFileName, address, keyPath string, ip net.IP) {
	if InstallerType == Apt {
		var keyPart string
		if keyPath == "" {
			keyPart = " trusted=yes"
		} else {
			keyPart = fmt.Sprintf(" signed-by=%s", keyPath)
		}
		RunCommandOn(fmt.Sprintf("echo \"deb [arch=amd64 %s] %s\" | sudo tee /etc/apt/sources.list.d/%s.list",
			keyPart, address, repoFileName), ip, true)
	} else if InstallerType == Yum || InstallerType == Dnf {
		var gpgCheck string
		var gpgKey string
		if keyPath == "" {
			gpgCheck = "0"
			gpgKey = ""
		} else {
			gpgCheck = "1"
			gpgKey = fmt.Sprintf("\ngpgkey=file://%s", gpgKey)
		}
		RunCommandOn(fmt.Sprintf("sudo bash -c 'echo -e \""+
			"[%s]\nname=%s\nbaseurl=%s\nenabled=1\ngpgcheck=%s%s\n"+
			"\" > /etc/yum.repos.d/%s.repo'", shortname, name,
			strings.ReplaceAll(address, "$", "\\$"), gpgCheck, gpgKey, repoFileName), ip, true)
	}
}

func UpdateRepos(ip net.IP) {
	util.StartSpinner(fmt.Sprintf("Updating repos on \"%s\"", ip))
	var cmd string
	if InstallerType == Apt {
		cmd = "sudo apt-get update -y"
	} else if InstallerType == Yum {
		cmd = "sudo yum check-update -y; sudo yum makecache fast -y"
	} else if InstallerType == Dnf {
		cmd = "sudo dnf check-update -y; sudo dnf makecache --refresh"
	}
	RunCommandOn(cmd, ip, true)
	util.StopSpinner("", logsymbols.Success)
}

func RemoveRelatedRepoFiles(name string, ip net.IP) {
	var cmd string
	if InstallerType == Apt {
		cmd = "sudo rm -f /etc/apt/sources.list.d/%s*.list"
	} else if InstallerType == Yum || InstallerType == Dnf {
		cmd = "sudo rm -f /etc/yum.repos.d/%s*.repo"
	}
	RunCommandOn(fmt.Sprintf(cmd, name), ip, true)
}

func RemovePackage(p string, ip net.IP) {
	installed, _ := PackageInstalledOn(p, ip)
	if installed {
		util.StartSpinner(fmt.Sprintf("Removing %s package on \"%s\"", p, ip))
		var cmd string
		if InstallerType == Apt {
			cmd = "sudo apt-get purge -y %s --allow-change-held-packages"
		} else if InstallerType == Yum {
			cmd = "sudo yum remove -y %s"
		} else if InstallerType == Dnf {
			cmd = "sudo dnf remove -y %s"
		}
		RunCommandOn(fmt.Sprintf(cmd, p), ip, true)
		if InstallerType == Apt {
			cmd = "sudo dpkg -P %s"
		} else if InstallerType == Yum || InstallerType == Dnf {
			cmd = "sudo rpm -e --nodeps %s || true"
		}
		RunCommandOn(fmt.Sprintf(cmd, p), ip, true)
		util.StopSpinner("", logsymbols.Success)
	}
}

func InstallPackage(ps string, ip net.IP) {
	psArr := strings.Fields(ps)
	var pCombined []string
	var pCombinedDowngraded []string
	for _, p := range psArr {
		var v string
		if strings.Contains(p, "=") {
			pSplit := strings.Split(p, "=")
			p = pSplit[0]
			v = pSplit[1]
			installed, installedVer := PackageInstalledOn(p, ip)
			if installed && strings.HasPrefix(installedVer, v) {
				log.Debugf("\"%s=%s\" is already installed on \"%s\". Skipping...", p, v, ip.String())
				continue
			}
			var cmd string
			var pkgVerCombineChar string
			if InstallerType == Apt {
				pkgVerCombineChar = "="
				cmd = "sudo apt list -a %s 2>/dev/null | cut -d '[' -f1 | grep %s" +
					" | head -1 | xargs | cut -d ' ' -f2"
			} else if InstallerType == Yum {
				pkgVerCombineChar = "-"
				cmd = "sudo yum list %s --showduplicates 2>/dev/null | grep %s" +
					" | tail -1 | xargs | cut -d ' ' -f2 | cut -d ':' -f2 | cut -d '-' -f1"
			} else if InstallerType == Dnf {
				pkgVerCombineChar = "-"
				cmd = "sudo dnf list %s --showduplicates 2>/dev/null | grep %s" +
					" | tail -1 | xargs | cut -d ' ' -f2 | cut -d ':' -f2 | cut -d '-' -f1"
			}
			exactVer := RunCommandOn(fmt.Sprintf(cmd, p, v), ip, true)
			if exactVer == "" {
				Exit(fmt.Sprintf("Version \"%s\" for \"%s\" was not found", v, p), 1)
			}
			exactSemVer := getSemVer(exactVer)
			var installedSemVer *version.Version
			if installed {
				installedSemVer = getSemVer(installedVer)
			} else {
				installedSemVer = getSemVer("0.0.0")
			}
			if exactSemVer.LessThan(installedSemVer) {
				pCombinedDowngraded = append(pCombinedDowngraded, fmt.Sprintf("%s%s%s", p, pkgVerCombineChar, exactVer))
			} else {
				pCombined = append(pCombined, fmt.Sprintf("%s%s%s", p, pkgVerCombineChar, exactVer))
			}
		} else {
			var cmd string
			if InstallerType == Apt {
				cmd = "sudo apt list -a %s 2>/dev/null | grep installed | wc -l"
			} else if InstallerType == Yum {
				cmd = "sudo yum list installed 2>/dev/null | grep ^%s | wc -l"
			} else if InstallerType == Dnf {
				cmd = "sudo dnf list installed 2>/dev/null | grep ^%s | wc -l"
			}
			installed := RunCommandOn(fmt.Sprintf(cmd, p), ip, true) == "1"
			if installed {
				log.Debugf("\"%s\" is already installed on \"%s\". Skipping...", p, ip.String())
				continue
			}
			pCombined = append(pCombined, p)
		}
	}
	var cmd string
	if len(pCombined) > 0 {
		if InstallerType == Apt {
			cmd = "sudo apt-get install -f -y --allow-unauthenticated --allow-downgrades " +
				"-o DPkg::Options::=\"--force-confnew\" %s"
		} else if InstallerType == Yum {
			cmd = "sudo yum install -y --setopt=obsoletes=0 %s"
		} else if InstallerType == Dnf {
			cmd = "sudo dnf install -y --setopt=obsoletes=0 %s"
		}
		util.StartSpinner(fmt.Sprintf("Installing \"%s\" on \"%s\"", strings.Join(pCombined, " "), ip))
		RunCommandOn(fmt.Sprintf(cmd, strings.Join(pCombined, " ")), ip, true)
		util.StopSpinner("", logsymbols.Success)
	}
	if len(pCombinedDowngraded) > 0 {
		if InstallerType == Apt {
			cmd = "sudo apt-get install -f -y --allow-unauthenticated --allow-downgrades " +
				"-o DPkg::Options::=\"--force-confnew\" %s"
		} else if InstallerType == Yum {
			cmd = "sudo yum downgrade -y --setopt=obsoletes=0 %s"
		} else if InstallerType == Dnf {
			cmd = "sudo dnf downgrade -y --setopt=obsoletes=0 %s"
		}
		util.StartSpinner(fmt.Sprintf("Downgrading \"%s\" on \"%s\"", strings.Join(pCombinedDowngraded, " "), ip))
		RunCommandOn(fmt.Sprintf(cmd, strings.Join(pCombinedDowngraded, " ")), ip, true)
		util.StopSpinner("", logsymbols.Success)
	}
}

func getSemVer(ver string) (semVer *version.Version) {
	var err error
	if strings.Contains(ver, ":") {
		semVer, err = version.NewVersion(strings.Split(ver, ":")[1])
	} else {
		semVer, err = version.NewVersion(ver)
	}
	if err != nil {
		Exit(err.Error(), 1)
	}
	return semVer
}

func InstallPackageNoStart(ps string, ip net.IP) {
	RunCommandOn("echo -e '#!/bin/sh\nexit 101' | sudo tee -a /usr/sbin/policy-rc.d", ip, true)
	RunCommandOn("sudo chmod +x /usr/sbin/policy-rc.d", ip, true)
	InstallPackage(ps, ip)
	RunCommandOn("sudo rm -f /usr/sbin/policy-rc.d", ip, true)
}

func LockPackageVersion(pkg string, ip net.IP) {
	var cmd string
	if InstallerType == Apt {
		cmd = "sudo apt-mark hold %s"
	} else if InstallerType == Yum {
		cmd = "sudo yum versionlock add %s"
	} else if InstallerType == Dnf {
		cmd = "sudo dnf versionlock add %s"
	}
	RunCommandOn(fmt.Sprintf(cmd, pkg), ip, true)
}

func IsSelinuxEnabled(ip net.IP) bool {
	out := RunCommandOn("sudo sestatus | grep -i \"selinux status\" | awk -F: '{ print $2 }' | xargs", ip, true)
	return out == "enabled"
}

func ChangeServiceStatus(service, status string, retryCount int, ip net.IP) {
	util.StartSpinner(fmt.Sprintf("\"%s\" service %s on processing", service, status))
	if retryCount < 1 {
		retryCount = 1
	}
	var err error
	for retryCount > 0 {
		_, err = runCommandOnReturnErr(fmt.Sprintf("sudo service %s %s", service, status), ip, true, true)
		if err == nil {
			break
		} else if retryCount > 1 {
			log.Debugf("\"%s %s\" is failed. It will be retried in 15 seconds", service, status)
		}
		retryCount--
		time.Sleep(15 * time.Second)
	}
	if err != nil {
		Exit(fmt.Sprintf("%s %s is failed on \"%s\"", service, status, ip), 1)
	}
	util.StopSpinner(fmt.Sprintf("%s %s is successful.", service, status), logsymbols.Success)
}

func IsFolderExists(dir string) bool {
	return IsFolderExistsOn(dir, nil)
}

func IsFolderExistsOn(dir string, ip net.IP) bool {
	command := fmt.Sprintf("[ -d %s ] && echo 1 || echo 0", dir)
	var output int
	if ip != nil {
		out := RunCommandOn(command, ip, true)
		output, _ = strconv.Atoi(out)
	} else {
		out := RunCommand(command, true)
		output, _ = strconv.Atoi(out)
	}
	return output != 0
}

func GetMd5(file string) string {
	return GetMd5On(file, nil)
}

func GetMd5On(file string, ip net.IP) string {
	command := fmt.Sprintf("md5=$(md5sum %s | awk '{print \"-n \"$1}')", file)
	if ip != nil {
		return RunCommandOn(command, ip, true)
	} else {
		return RunCommand(command, true)
	}
}

func IsFileExists(md5toCheck, path string) bool {
	return IsFileExistsOn(md5toCheck, path, nil)
}

func IsFileExistsOn(md5toCheck, dir string, ip net.IP) bool {
	command := fmt.Sprintf("[ -f %s ] && echo 1 || echo 0", dir)
	var output int
	if ip != nil {
		out := RunCommandOn(command, ip, true)
		output, _ = strconv.Atoi(out)
	} else {
		out := RunCommand(command, true)
		output, _ = strconv.Atoi(out)
	}
	if output != 0 && md5toCheck != "" {
		command = fmt.Sprintf("md5=$(md5sum %s | awk '{print \"-n \"$1}') && "+
			"if [ \"%s\" == \"$md5\" ]; then echo 1; else echo 0; fi", dir, md5toCheck)

		if ip != nil {
			out := RunCommandOn(command, ip, true)
			output, _ = strconv.Atoi(out)
		} else {
			out := RunCommand(command, true)
			output, _ = strconv.Atoi(out)
		}
	}
	return output != 0
}

func CreateFile(data []byte, dstFile string, ip net.IP) {
	folder := dstFile[:strings.LastIndexAny(dstFile, "/")]
	fileName := dstFile[strings.LastIndexAny(dstFile, "/")+1:]
	tempDst := fmt.Sprintf("/tmp/%s", fileName)
	if ip == nil {
		err := os.MkdirAll(folder, os.FileMode(0777))
		if err != nil {
			log.Debugf("Error occurred while creating \"%s\" folder", folder)
			Exit(err.Error(), 1)
		}
		err = os.WriteFile(tempDst, data, os.FileMode(0666))
		if err != nil {
			log.Debugf("Error occurred while writing \"%s\" file", dstFile)
			Exit(err.Error(), 1)
		}
		_, err = localRun(fmt.Sprintf("sudo mv %s %s", tempDst, dstFile), true)
		if err != nil {
			log.Debugf("Error occurred while moving \"%s\" file", dstFile)
			Exit(err.Error(), 1)
		}
	} else {
		cmd := fmt.Sprintf("mkdir -p %s", folder)
		if !strings.HasPrefix(dstFile, "/home") {
			cmd = fmt.Sprintf("sudo %s", cmd)
		}
		RunCommandOn(cmd, ip, true)
		err := conn.SendFile(ip, bytes.NewReader(data), tempDst)
		if err != nil {
			log.Debugf("Error occurred while sending \"%s\" file to %s", dstFile, ip)
			Exit(err.Error(), 1)
		}
		_, err = runCommandOnReturnErr(fmt.Sprintf("sudo mv %s %s", tempDst, dstFile), ip, true, true)
		if err != nil {
			log.Debugf("Error occurred while moving \"%s\" file", dstFile)
			Exit(err.Error(), 1)
		}
	}
}

func TransferFile(srcPath, dstPath string, from, to net.IP) (err error) {
	folder := dstPath[:strings.LastIndexAny(dstPath, "/")]
	if from.Equal(to) {
		RunCommandOn(fmt.Sprintf("mkdir -p %s", folder), from, true)
		RunCommandOn(fmt.Sprintf("sudo cp %s %s", srcPath, dstPath), from, true)
	} else {
		fromNode := conn.Nodes[from.String()]
		if strings.Contains(srcPath, "$HOME") {
			srcPath = strings.ReplaceAll(srcPath, "$HOME", getHomePath(fromNode.SSHUser))
		}
		toNode := conn.Nodes[to.String()]
		if strings.Contains(dstPath, "$HOME") {
			dstPath = strings.ReplaceAll(dstPath, "$HOME", getHomePath(fromNode.SSHUser))
		}
		RunCommandOn(fmt.Sprintf("mkdir -p %s", folder), to, true)
		cmd := ""
		if sshPassNeeded(from, to) {
			cmd = fmt.Sprintf("%s sshpass -p %s", cmd, toNode.SSHPass)
		}
		RunCommandOn(fmt.Sprintf("%s scp "+
			"-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -r %s %s:%s",
			cmd, srcPath, toNode, dstPath), from, true)
	}
	return nil
}

func getHomePath(user string) string {
	if user == "root" {
		return "/root"
	} else {
		return fmt.Sprintf("/home/%s", user)
	}
}

func sshPassNeeded(from, to net.IP) bool {
	output := RunCommandOn(fmt.Sprintf("ssh -o PasswordAuthentication=no %s /bin/true >nul 2>&1; echo $? | xargs",
		to.String()), from, true)
	if output == "0" {
		return false
	}
	return true
}

func GetFileNamesInDir(dir string) ([]string, error) {
	var returnArr []string
	if RemoteNode == nil || RemoteNode.IP == nil {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				returnArr = append(returnArr, e.Name())
			}
		}
	} else {
		output := RunCommand(fmt.Sprintf("ls -p %s | grep -v /", dir), true)
		returnArr = append(returnArr, strings.Split(strings.TrimSuffix(output, "\n"), "\n")...)
	}
	return returnArr, nil
}

func CreateFolderIfNotExists(folder string) error {
	if !IsFolderExists(folder) {
		var err error
		if RemoteNode == nil || RemoteNode.IP == nil {
			err = os.MkdirAll(folder, os.FileMode(0777))
		} else {
			RunCommandOn(fmt.Sprintf("mkdir -p %s", folder), RemoteNode.IP, true)
		}
		if err != nil {
			log.Debugf("Error occurred while creating \"%s\" folder", folder)
			return err
		}
	}
	return nil
}

func ReadFile(path string, ip net.IP) (data []byte, err error) {
	if ip == nil {
		return os.ReadFile(path)
	} else {
		returnStr, err := runCommandOnReturnErr(fmt.Sprintf("sudo cat %s", path), ip, true, true)
		return []byte(returnStr), err
	}
}

func CommandExists(command string) bool {
	output := RunCommand(fmt.Sprintf("command -v %s | xargs", command), true)
	return output != ""
}

func PackageInstalledOn(p string, ip net.IP) (installed bool, version string) {
	var cmd string
	var versionIndex int
	if InstallerType == Apt {
		versionIndex = 2
		cmd = "dpkg --list %s | tail -n 1"
	} else if InstallerType == Yum {
		versionIndex = 1
		cmd = "sudo yum list installed 2>/dev/null | grep ^%s | head -1"
	} else if InstallerType == Dnf {
		versionIndex = 1
		cmd = "sudo dnf list installed 2>/dev/null | grep ^%s | head -1"
	}
	returnStr := strings.TrimSpace(RunCommandOn(fmt.Sprintf(cmd, p), ip, true))
	if returnStr == "" || strings.Contains(returnStr, "no packages") ||
		len(strings.Fields(returnStr)) < (versionIndex+1) {

		return false, ""
	}
	return true, strings.Fields(returnStr)[versionIndex]
}

func MountISO(mountPath, isoPath string, ip net.IP) {
	RunCommandOn(fmt.Sprintf("sudo mkdir -p %s && sudo mount -t iso9660 -o loop %s %s",
		mountPath, isoPath, mountPath), ip, true)
}

func UmountISO(mountPath string, ip net.IP) {
	_, err := runCommandOnReturnErr(fmt.Sprintf("mountpoint %s", mountPath), ip, true, true)
	if err == nil { // means there is a mount point
		RunCommandOn(fmt.Sprintf("sudo umount %s", mountPath), ip, true)
	}
}

func ThrowIfError(err error, code int) {
	if err != nil {
		Exit(err.Error(), code)
	}
}

func Exit(message string, code int) {
	if util.GetSpinner() != nil {
		suffix := util.GetSpinner().Suffix
		if code != 0 {
			util.StopSpinner(suffix[1:], logsymbols.Error)
		} else {
			util.StopSpinner(suffix[1:], logsymbols.Success)
		}
	}
	if message != "" {
		if code == 0 {
			color.Green(message)
		} else {
			color.Red(message)
		}
	}
	for addr, exists := range sudoersPrevExistsMap {
		ip := net.ParseIP(addr)
		if !exists {
			RemoveFromSudoers(ip)
		}
		RunCommandOn("sudo rm -f /usr/sbin/policy-rc.d || true", ip, true)
		RunCommandOn(fmt.Sprintf("rm -rf $HOME/%s/%s", constant.CfgRootFolder, constant.TmpFolder), ip, true)
	}
	conn.CloseSSHSessions()
	fmt.Print("\033[?25h") // make cursor visible
	os.Exit(code)
}
