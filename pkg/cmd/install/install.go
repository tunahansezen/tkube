package install

import (
	"com.github.tunahansezen/tkube/pkg/cmd"
	"com.github.tunahansezen/tkube/pkg/core"
	"github.com/spf13/cobra"
)

const (
	fKubeVersion       = "kube-version"
	fMultiMaster       = "multi-master"
	DefaultMultiMaster = false
)

var (
	kubeVersion string
	multiMaster bool
)

// Cmd represents the purge command
var Cmd = &cobra.Command{
	Use:   "install",
	Short: "Install kubernetes",
	Long:  `Install kubernetes`,
	PreRun: func(cmd *cobra.Command, args []string) {
		core.PreRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		core.MasterConfigs()
	},
}

func init() {
	cmd.RootCmd.AddCommand(Cmd)
	Cmd.Flags().StringVarP(&kubeVersion, fKubeVersion, "", "", "Kubernetes version")
	Cmd.Flags().BoolVarP(&multiMaster, fMultiMaster, "", DefaultMultiMaster, "Multi-Master Kubernetes installation")
}
