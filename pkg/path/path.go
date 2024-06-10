package path

import (
	conn "com.github.tunahansezen/tkube/pkg/connection"
	"com.github.tunahansezen/tkube/pkg/constant"
	"com.github.tunahansezen/tkube/pkg/os"
	"fmt"
	"net"
)

var (
	homePath          string
	tkubeMainDir      string
	tkubeCfgDir       string
	tkubeResourcesDir string
	tkubeTmpDir       string
)

func GetTKubeMainDir() string {
	return tkubeMainDir
}

func GetTKubeCfgDir() string {
	return tkubeCfgDir
}

func GetTKubeResourcesDir() string {
	return tkubeResourcesDir
}

func GetTKubeTmpDir(ip net.IP) string {
	return fmt.Sprintf(fmt.Sprintf("/home/%s/%s/%s", conn.Nodes[ip.String()].SSHUser,
		constant.CfgRootFolder, constant.TmpFolder))
}

func CalculatePaths() {
	homePath = os.RunCommand("echo $HOME", true)
	tkubeMainDir = fmt.Sprintf("%s/%s", homePath, constant.CfgRootFolder)
	tkubeCfgDir = fmt.Sprintf("%s/%s", tkubeMainDir, constant.CfgFolder)
	tkubeResourcesDir = fmt.Sprintf("%s/%s", tkubeMainDir, constant.ResourcesFolder)
	tkubeTmpDir = fmt.Sprintf("%s/%s", tkubeMainDir, constant.TmpFolder)
}
