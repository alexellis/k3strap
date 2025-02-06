package main

import (
	"os"

	"github.com/alexellis/k3sup/cmd"
	"github.com/alexellis/k3sup/pkg"
	"github.com/spf13/cobra"
)

func main() {

	cmdInstall := cmd.MakeInstall()
	cmdVersion := cmd.MakeVersion()
	cmdJoin := cmd.MakeJoin()
	cmdUpdate := cmd.MakeUpdate()
	cmdReady := cmd.MakeReady()
	cmdPlan := cmd.MakePlan()
	cmdNodeToken := cmd.MakeNodeToken()
        cmdReset := cmd.MakeReset()

	printk3supASCIIArt := cmd.PrintK3supASCIIArt

	var rootCmd = &cobra.Command{
		Use: "k3sup",
		Run: func(cmd *cobra.Command, args []string) {
			printk3supASCIIArt()
			cmd.Help()
		},
		Example: `  # Install k3s on a server with embedded etcd
  k3sup install \
    --cluster \
    --host $SERVER_1 \
    --user $SERVER_1_USER \
    --k3s-channel stable

  # Join a second server
  k3sup join \
    --server \
    --host $SERVER_2 \
    --user $SERVER_2_USER \
    --server-host $SERVER_1 \
    --server-user $SERVER_1_USER \
    --k3s-channel stable

  # Join an agent to the cluster
  k3sup join \
    --host $SERVER_1 \
    --user $SERVER_1_USER \
    --k3s-channel stable
  
` + pkg.SupportMessageShort,
	}

	rootCmd.AddCommand(cmdInstall)
	rootCmd.AddCommand(cmdVersion)
	rootCmd.AddCommand(cmdJoin)
	rootCmd.AddCommand(cmdUpdate)
	rootCmd.AddCommand(cmdReady)
	rootCmd.AddCommand(cmdPlan)
	rootCmd.AddCommand(cmdNodeToken)
        rootCmd.AddCommand(cmdReset)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
