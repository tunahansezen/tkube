package cmd

import (
	"com.github.tunahansezen/tkube/pkg/core"
	"com.github.tunahansezen/tkube/pkg/os"
	"github.com/spf13/cobra"
)

const (
	versionTemplate = `{{printf "tkube v%s" .Version}}
`
	fDebug             = "debug"
	fTrace             = "trace"
	fsDebug            = "d"
	fAuthMap           = "auth-map"
	fRemoteNode        = "remote"
	fDockerVersion     = "docker"
	fContainerdVersion = "containerd"
	fEtcdVersion       = "etcd"
	fKubeVersion       = "kube"
	fCalicoVersion     = "calico"
	fHelmVersion       = "helm"
	fDockerPrune       = "docker-prune"
	fIso               = "iso"
	fSkipImageLoad     = "skip-image-load"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:          "tkube",
	Short:        "tkube multi-master kubernetes installer",
	Long:         `tkube multi-master kubernetes installer`,
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute(version string) {
	RootCmd.Version = version
	//core.SetInstallerVersion(version)
	RootCmd.InitDefaultVersionFlag()
	RootCmd.Flag("version").Usage = "version for tkube"
	//core.RunningUser, _ = os.RunCommand("whoami", true)
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(err.Error(), 1)
	}
}

func init() {
	RootCmd.SetVersionTemplate(versionTemplate)
	RootCmd.CompletionOptions.HiddenDefaultCmd = true
	RootCmd.CompletionOptions.DisableDescriptions = true

	RootCmd.PersistentFlags().BoolVarP(&core.Debug, fDebug, fsDebug, false, "debug logging for tkube")
	RootCmd.PersistentFlags().BoolVarP(&core.Trace, fTrace, "", false, "trace logging for tkube")
	RootCmd.PersistentFlags().StringVarP(&core.AuthMapStr, fAuthMap, "", "", "SSH auth information")
	RootCmd.PersistentFlags().IPVarP(&os.RemoteNodeIP, fRemoteNode, "", nil,
		"if node defined, remote installation will be processed")
	RootCmd.PersistentFlags().StringVarP(&core.DockerVersion, fDockerVersion, "", core.DefaultDockerVersion,
		"docker version")
	RootCmd.PersistentFlags().StringVarP(&core.ContainerdVersion, fContainerdVersion, "", core.DefaultContainerdVersion,
		"containerd version")
	RootCmd.PersistentFlags().StringVarP(&core.EtcdVersion, fEtcdVersion, "", core.DefaultEtcdVersion,
		"etcd version")
	RootCmd.PersistentFlags().StringVarP(&core.KubeVersion, fKubeVersion, "", core.DefaultKubeVersion,
		"kubernetes version")
	RootCmd.PersistentFlags().StringVarP(&core.CalicoVersion, fCalicoVersion, "", core.DefaultCalicoVersion,
		"calico version")
	RootCmd.PersistentFlags().StringVarP(&core.HelmVersion, fHelmVersion, "", core.DefaultHelmVersion,
		"helm version")
	RootCmd.PersistentFlags().BoolVarP(&core.DockerPrune, fDockerPrune, "", core.DefaultDockerPrune,
		"prune all docker images and other data")
	RootCmd.PersistentFlags().StringVarP(&core.IsoPath, fIso, "", "", "ISO file path for offline installation")
	RootCmd.PersistentFlags().BoolVarP(&core.SkipImageLoad, fSkipImageLoad, "", core.DefaultSkipImageLoad,
		"prune all docker images and other data")
}
