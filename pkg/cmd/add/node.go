package add

import (
	cfg "com.github.tunahansezen/tkube/pkg/config"
	"com.github.tunahansezen/tkube/pkg/config/model"
	"com.github.tunahansezen/tkube/pkg/core"
	"com.github.tunahansezen/tkube/pkg/os"
	"fmt"
	"github.com/spf13/cobra"
)

const (
	fHostname = "hostname"
)

var (
	hostname string
)

// nodeCmd represents the add worker command
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Add node",
	Long:  `Add node to the cluster`,
	PreRun: func(cmd *cobra.Command, args []string) {
		core.PreRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		node := cfg.DeploymentCfg.GetNodeWithHostname(hostname)
		if node == nil {
			os.Exit(fmt.Sprintf("Node with \"%s\" hostname not found in deployment config", hostname), 1)
		}
		var nodes model.KubeNodes
		nodes.Nodes = []model.KubeNode{*node}
		core.Install(nodes, false)
	},
}

func init() {
	Cmd.AddCommand(nodeCmd)
	nodeCmd.Flags().StringVarP(&hostname, fHostname, "", "", "Node hostname")
}
