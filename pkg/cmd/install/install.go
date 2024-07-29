package install

import (
	"com.github.tunahansezen/tkube/pkg/cmd"
	cfg "com.github.tunahansezen/tkube/pkg/config"
	"com.github.tunahansezen/tkube/pkg/config/model"
	"com.github.tunahansezen/tkube/pkg/core"
	"github.com/spf13/cobra"
)

const (
	fSkipWorkers = "skip-workers"
)

var (
	skipWorkers bool
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
		kubeNodes := cfg.DeploymentCfg.GetKubeNodes()
		if skipWorkers {
			kubeNodes = cfg.DeploymentCfg.GetMasterKubeNodes()
		}
		var nodes model.KubeNodes
		nodes.Nodes = kubeNodes
		core.Install(nodes)
	},
}

func init() {
	cmd.RootCmd.AddCommand(Cmd)
	Cmd.Flags().BoolVarP(&skipWorkers, fSkipWorkers, "", false, "Skip worker nodes kubernetes installation")
}
