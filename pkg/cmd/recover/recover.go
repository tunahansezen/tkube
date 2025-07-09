package add

import (
	"com.github.tunahansezen/tkube/pkg/cmd"
	"com.github.tunahansezen/tkube/pkg/os"
	"fmt"
	"github.com/spf13/cobra"
)

const (
	cRecover = "recover"
)

// Cmd represents the recover command
var Cmd = &cobra.Command{
	Use:   cRecover,
	Short: "Recover master nodes",
	Long:  `Recover master nodes to cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			os.Exit(fmt.Sprintf("Error occurred while calling help for \"%s\"", cRecover), 1)
		}
	},
}

func init() {
	cmd.RootCmd.AddCommand(Cmd)
}
