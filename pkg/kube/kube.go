package kube

import (
	"com.github.tunahansezen/tkube/pkg/os"
	"com.github.tunahansezen/tkube/pkg/util"
	"fmt"
	"github.com/guumaster/logsymbols"
	"github.com/hashicorp/go-version"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
)

type kubeType int

const (
	Pod kubeType = iota
	Deployment
	StatefulSet
	DaemonSet
	Replicaset
)

func (k kubeType) String() string {
	switch k {
	case Pod:
		return "pod"
	case Deployment:
		return "deployments.app"
	case StatefulSet:
		return "statefulsets.app"
	case DaemonSet:
		return "daemonsets.app"
	case Replicaset:
		return "replicasets.app"
	}
	return "unknown"
}

type Node struct {
	Name   string
	IP     net.IP
	Ready  bool
	Master bool
}

func CreateCertKey(kubeVersion string, ip net.IP) (certKey string) {
	kubeSemVer, _ := version.NewVersion(kubeVersion)
	kubeadmCertsSemVer, _ := version.NewVersion("1.20.0")
	if kubeSemVer.GreaterThanOrEqual(kubeadmCertsSemVer) {
		certKey = os.RunCommandOn("sudo kubeadm certs certificate-key", ip, true)
	} else {
		certKey = os.RunCommandOn("sudo kubeadm alpha certs certificate-key", ip, true)
	}
	return certKey
}

func GetNodes(masterNodeIP net.IP) []Node {
	output := os.RunCommandOn("kubectl get nodes -o wide --no-headers", masterNodeIP, true)
	var returnNodes []Node
	for i, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if (i == 0) && (strings.Contains(line, "was refused")) {
			os.Exit("kube server not running", 1)
		}
		fields := strings.Fields(line)
		name := fields[0]
		ready := false
		if strings.Contains(fields[1], "Ready") {
			ready = false
		}
		master := false
		if strings.Contains(fields[2], "master") || strings.Contains(fields[2], "control-plane") {
			master = true
		}
		ip := fields[5]
		returnNodes = append(returnNodes, Node{Name: name, Ready: ready, Master: master, IP: net.ParseIP(ip)})
	}
	return returnNodes
}

func GetPodNames(podKeyword string, namespace string) []string {
	var returnArr []string
	output := os.RunCommand(fmt.Sprintf("kubectl get pods -n %s --ignore-not-found | grep %s | awk '{print $1}'",
		namespace, podKeyword), true)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		returnArr = append(returnArr, line)
	}
	return returnArr
}

func GetActiveNamespaces(namespaceKeyword string) []string {
	var returnArr []string
	output := os.RunCommand(fmt.Sprintf("kubectl get pods --all-namespaces | grep %s | "+
		"awk '{print $1}' | awk '!seen[$0]++'", namespaceKeyword), true)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		returnArr = append(returnArr, line)
	}
	return returnArr
}

func Exec(podName string, namespace string, shell string, command string, container string) string {
	containerParam := ""
	if container != "" {
		containerParam = fmt.Sprintf(" -c %s", container)
	}
	output := os.RunCommand(fmt.Sprintf("kubectl exec -i -n %s %s%s -- %s -c '%s'",
		namespace, podName, containerParam, shell, command), true)
	return output
}

func Copy(podName string, namespace string, srcPath string, dstPath string, container string) {
	containerParam := ""
	if container != "" {
		containerParam = fmt.Sprintf(" -c %s", container)
	}
	os.RunCommand(fmt.Sprintf("kubectl cp %s %s:%s%s -n %s", srcPath, podName, dstPath, containerParam,
		namespace), true)
}

func CreateNamespace(namespace string) {
	output := os.RunCommand("kubectl get namespaces -o jsonpath=\"{.items[*].metadata.name}\"", true)
	namespaces := strings.Split(output, " ")
	if !slices.Contains(namespaces, namespace) {
		os.RunCommand(fmt.Sprintf("kubectl create namespace %s", namespace), true)
	}
}

func isSecretExists(secretName string, namespace string) bool {
	out := os.RunCommand(fmt.Sprintf("kubectl get secrets %s --no-headers -n %s --ignore-not-found | wc -l",
		secretName, namespace), true)
	output, _ := strconv.Atoi(out)
	return output != 0
}

func SetEnv(name, namespace, env string) {
	os.RunCommand(fmt.Sprintf("kubectl set env %s -n %s '%s'", name, namespace, env), true)
}

func WaitUntilPodsRunning(namespaces []string) {
	notReadyCount := notReadyPodCount(namespaces)
	msg := ""
	if len(namespaces) > 0 {
		msg = fmt.Sprintf("Installation continues for %s pods at namespace(s): \"%s\"",
			"%d", strings.Join(namespaces, ", "))
	} else {
		msg = "Installation continues for %d pods"
	}
	util.StartSpinner(fmt.Sprintf(msg, notReadyCount))
	for notReadyCount != 0 {
		time.Sleep(util.WaitSleep)
		notReadyCount = notReadyPodCount(namespaces)
		util.UpdateSpinner(fmt.Sprintf(msg, notReadyCount))
	}
	util.StopSpinner(fmt.Sprintf("All pods running at namespace(s): %s", strings.Join(namespaces, ", ")),
		logsymbols.Success)
}

func WaitUntilPodsRunningWithName(podNames []string, namespace string) {
	notRunning := podNames
	util.StartSpinner(fmt.Sprintf("Waiting %s pods to be ready", strings.Join(notRunning, ", ")))
	for len(notRunning) > 0 {
		notRunning = make([]string, 0)
		for _, pod := range podNames {
			line := os.RunCommand(fmt.Sprintf("kubectl get pod --ignore-not-found --no-headers -n %s %s",
				namespace, pod), true)
			if line == "" {
				notRunning = append(notRunning, pod)
			}
			s1 := strings.Fields(line)
			if len(s1) < 3 {
				continue
			}
			ready := s1[1]
			s2 := strings.Split(ready, "/")
			if len(s2) != 2 {
				continue
			}
			runningContainer, _ := strconv.Atoi(s2[0])
			totalContainer, _ := strconv.Atoi(s2[1])
			if runningContainer != totalContainer {
				notRunning = append(notRunning)
			}
		}
		if len(notRunning) > 0 {
			util.UpdateSpinner(fmt.Sprintf("Waiting %s pods to be ready", strings.Join(notRunning, ", ")))
			time.Sleep(10 * time.Second)
		}
	}
	util.StopSpinner(fmt.Sprintf("%s pods running", strings.Join(podNames, ", ")), logsymbols.Success)
}

func notReadyPodCount(namespaces []string) int {
	namespacesCmd := ""
	if len(namespaces) > 0 {
		namespacesCmd = fmt.Sprintf(" | grep -E '%s'", strings.Join(namespaces, "|"))
	} else {
		namespacesCmd = " | grep -v kube-system"
	}
	notReadyCount := 0
	output, err := os.RunCommandReturnError(fmt.Sprintf("kubectl get pods --no-headers -A%s", namespacesCmd), true)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		s1 := strings.Fields(line)
		if len(s1) < 3 {
			continue
		}
		ready := s1[2]
		s2 := strings.Split(ready, "/")
		if len(s2) != 2 {
			continue
		}
		runningContainer, _ := strconv.Atoi(s2[0])
		totalContainer, _ := strconv.Atoi(s2[1])
		if runningContainer != totalContainer {
			notReadyCount++
		}
	}
	return notReadyCount
}

func WaitUntilAllPodsDeleted(namespaces []string) {
	var remaining = podCount(namespaces)
	util.StartSpinner(fmt.Sprintf("Remaining pod count: %d", remaining))
	for remaining > 0 {
		time.Sleep(util.WaitSleep)
		remaining = podCount(namespaces)
		util.UpdateSpinner(fmt.Sprintf("Remaining pod count: %d", remaining))
	}
	util.StopSpinner("All pods deleted", logsymbols.Success)
}

func waitUntilAllPodsDeletedAtNamespace(namespace string) {
	var remaining = podAtNamespace(namespace)
	util.StartSpinner(fmt.Sprintf("Remaining pod count at \"%s\" namespace: %d", namespace, remaining))
	for remaining > 0 {
		time.Sleep(util.WaitSleep)
		remaining = podAtNamespace(namespace)
		util.UpdateSpinner(fmt.Sprintf("Remaining pod count at \"%s\" namespace: %d", namespace, remaining))
	}
	util.StopSpinner(fmt.Sprintf("All pods deleted at \"%s\" namespace", namespace), logsymbols.Success)

}

func podCount(namespaces []string) int {
	namespacesCmd := ""
	if len(namespaces) > 0 {
		namespacesCmd = fmt.Sprintf(" | grep -E '%s'", strings.Join(namespaces, "|"))
	}
	output := os.RunCommand(fmt.Sprintf("kubectl get pods --ignore-not-found --no-headers -A%s | "+
		"grep -v kube-system | wc -l", namespacesCmd), true)
	count, _ := strconv.Atoi(output)
	return count
}

func podAtNamespace(namespace string) int {
	output := os.RunCommand(fmt.Sprintf("kubectl get pods --ignore-not-found --no-headers -n %s | "+
		"grep -v kube-system | wc -l", namespace), true)
	count, _ := strconv.Atoi(output)
	return count
}

func DeleteAllPvAndPvc(namespaces []string) {
	if len(namespaces) == 0 {
		util.StartSpinner("Removing persistent volumes and claims")
		os.RunCommand("kubectl delete --all pvc,pv --all-namespaces", true)
		util.StopSpinner("All persistent volume and claims removed", logsymbols.Success)
	} else {
		util.StartSpinner(fmt.Sprintf("Removing persistent volumes and claims at namespaces: %s",
			strings.Join(namespaces, ", ")))
		for _, namespace := range namespaces {
			DeletePvAndPvcAtNamespace(namespace)
		}
		util.StopSpinner(fmt.Sprintf("All persistent volume and claims removed at namespaces: %s",
			strings.Join(namespaces, ", ")), logsymbols.Success)
	}
}

func DeletePvAndPvcAtNamespace(namespace string) {
	os.RunCommand(fmt.Sprintf("kubectl get pv --no-headers | grep -%s- | awk '{print $1}' | "+
		"xargs -r -L1 kubectl delete pv", namespace), true)
	os.RunCommand(fmt.Sprintf("kubectl delete pvc --all -n %s", namespace), true)
}

func DeleteSecrets(namespaces []string) {
	namespacesCmd := ""
	if len(namespaces) > 0 {
		namespacesCmd = fmt.Sprintf(" | grep -E '%s'", strings.Join(namespaces, "|"))
	}
	util.StartSpinner("Removing secrets")
	os.RunCommand(fmt.Sprintf("kubectl get secrets --no-headers --all-namespaces | grep -v kube-system | "+
		"grep -v default-token%s | awk '{print \"-n \"$1, $2}' | xargs -r -L1 kubectl delete secret", namespacesCmd),
		true)
	util.StopSpinner("All secrets removed", logsymbols.Success)
}

func ForceDeleteKube(t kubeType, namespaces []string) {
	namespacesCmd := ""
	if len(namespaces) > 0 {
		namespacesCmd = fmt.Sprintf(" | grep -E '%s'", strings.Join(namespaces, "|"))
	}
	output := os.RunCommand(fmt.Sprintf("kubectl get %s --ignore-not-found -A --no-headers | "+
		"grep -v kube-system%s || true", t, namespacesCmd), true)
	if output == "" {
		return
	}
	fmt.Printf("Force delete %s\n", t)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		fields := strings.Fields(line)
		namespace := fields[0]
		name := fields[1]
		fmt.Printf("Force deleting %s with name %s at namespace %s\n", t, name, namespace)
		if t == Pod {
			os.RunCommand(
				fmt.Sprintf("kubectl patch %s -n %s %s -p '{\"metadata\":{\"finalizers\":null}}' || true",
					t, namespace, name), true)
		}
		os.RunCommand(fmt.Sprintf("kubectl delete %s -n %s %s --grace-period=0 --force || true",
			t, namespace, name), true)
	}
}

func ForceDeleteAllPv(namespaces []string) {
	namespacesCmd := ""
	if len(namespaces) > 0 {
		namespacesCmd = fmt.Sprintf(" | grep -E '%s'", strings.Join(namespaces, "|"))
	}
	output := os.RunCommand(
		fmt.Sprintf("kubectl get pv --ignore-not-found -A --no-headers%s", namespacesCmd), true)
	if output == "" {
		return
	}
	fmt.Println("Force delete all pv")
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		fields := strings.Fields(line)
		name := fields[0]
		fmt.Printf("Force deleting pv with name %s\n", name)
		os.RunCommand(
			fmt.Sprintf("kubectl patch pv %s -p '{\"metadata\":{\"finalizers\":null}}'", name), true)
	}
}

func ForceDeleteAllPvc(namespaces []string) {
	namespacesCmd := ""
	if len(namespaces) > 0 {
		namespacesCmd = fmt.Sprintf(" | grep -E '%s'", strings.Join(namespaces, "|"))
	}
	output := os.RunCommand(
		fmt.Sprintf("kubectl get pvc --ignore-not-found -A --no-headers%s", namespacesCmd), true)
	if output == "" {
		return
	}
	fmt.Println("Force delete all pvc")
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		fields := strings.Fields(line)
		namespace := fields[0]
		name := fields[1]
		fmt.Printf("Force deleting pv with name %s at namespace %s\n", name, namespace)
		os.RunCommand(fmt.Sprintf("kubectl patch pv -n %s %s -p '{\"metadata\":{\"finalizers\":null}}'",
			namespace, name), true)
	}
}
