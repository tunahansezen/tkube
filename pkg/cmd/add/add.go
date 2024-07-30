package add

import (
	"com.github.tunahansezen/tkube/pkg/cmd"
	"com.github.tunahansezen/tkube/pkg/os"
	"fmt"
	"github.com/spf13/cobra"
)

const (
	cAdd = "add"
)

// Cmd represents the add command
var Cmd = &cobra.Command{
	Use:   cAdd,
	Short: "Add nodes",
	Long:  `Add nodes to cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			os.Exit(fmt.Sprintf("Error occurred while calling help for \"%s\"", cAdd), 1)
		}
	},
}

func init() {
	cmd.RootCmd.AddCommand(Cmd)
}
