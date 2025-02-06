package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexellis/k3sup/pkg"
	"github.com/spf13/cobra"
)

func MakePlan() *cobra.Command {
	command := &cobra.Command{
		Use:   "plan",
		Short: "Plan an installation of K3s.",
		Long: `Generate a bash script or plan of installation commands for K3s for a 
Highly Available (HA) Kubernetes cluster.

Examples JSON input file:

[{"hostname": "node-1", "ip": "192.168.128.102"},
{"hostname": "node-2", "ip": "192.168.128.103"},
{"hostname": "node-3", "ip": "192.168.128.104"}]

` + pkg.SupportMessageShort + `
`,
		Example: `  # Generate an installation script where the first
  # 3 available hosts are dedicated as servers, with a custom user.
  # The remaining hosts are added as agents.
  k3sup plan hosts.json --servers 3 --user ubuntu

  # Override the TLS SAN, for HA with 5 servers specified
  k3sup plan hosts.json --servers 5 --tls-san $SAN_IP
`,
		SilenceUsage: true,
	}

	command.Flags().Int("servers", 3, "Number of servers to use from the devices file")
	command.Flags().String("local-path", "kubeconfig", "Where to save the kubeconfig file")
	command.Flags().Bool("merge", false, `Merge the config with existing kubeconfig if it already exists.
Provide the --local-path flag with --merge if a kubeconfig already exists in some other directory`)
	command.Flags().String("context", "default", "Name of the kubeconfig context to use")
	command.Flags().String("user", "root", "Username for SSH login")

	command.Flags().String("ssh-key", "", "Path to the private key for SSH login")
	command.Flags().String("tls-san", "", "SAN for TLS certificates, can be a comma-separated list")
	command.Flags().String("server-k3s-extra-args", "", "Extra arguments to be passed into the k3s server")
	command.Flags().String("agent-k3s-extra-args", "", "Extra arguments to be passed into the k3s agent")

	// Background
	command.Flags().Bool("background", false, "Run the installation in the background for all agents/nodes after the first server is up")

	command.Flags().Int("limit", 0, "Maximum number of nodes to use from the devices file, 0 to use all devices")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("give a path to a JSON file containing a list of devices")
		}

		nodeLimit, _ := cmd.Flags().GetInt("limit")
		name := args[0]
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}

		background, _ := cmd.Flags().GetBool("background")

		var hosts []Host
		if err = json.Unmarshal(data, &hosts); err != nil {
			return err
		}

		serverK3sExtraArgs, _ := cmd.Flags().GetString("server-k3s-extra-args")
		agentK3sExtraArgs, _ := cmd.Flags().GetString("agent-k3s-extra-args")

		servers, _ := cmd.Flags().GetInt("servers")
		kubeconfig, _ := cmd.Flags().GetString("local-path")
		merge, _ := cmd.Flags().GetBool("merge")
		contextName, _ := cmd.Flags().GetString("context")
		user, _ := cmd.Flags().GetString("user")
		tlsSan, _ := cmd.Flags().GetString("tls-san")

		tlsSanStr := ""
		if len(tlsSan) > 0 {
			tlsSanStr = fmt.Sprintf(` \
--tls-san %s`, tlsSan)
		}

		sshKey, _ := cmd.Flags().GetString("ssh-key")

		sshKeySt := ""
		if len(sshKey) > 0 {
			sshKeySt = fmt.Sprintf(` \
--ssh-key %s`, sshKey)
		}

		bgStr := ""
		if background {
			bgStr = " &"
		}

		serversAdded := 0
		var primaryServer Host
		script := "#!/bin/sh\n\n"

		serverExtraArgsSt := ""
		if len(serverK3sExtraArgs) > 0 {
			serverExtraArgsSt = fmt.Sprintf(` \
--k3s-extra-args "%s"`, serverK3sExtraArgs)
		}
		agentExtraArgsSt := ""
		if len(agentK3sExtraArgs) > 0 {
			agentExtraArgsSt = fmt.Sprintf(` \
--k3s-extra-args "%s"`, agentK3sExtraArgs)
		}

		mergeStr := ""
		if merge {
			mergeStr = ` \
--merge`
		}

		for i, host := range hosts {
			if serversAdded == 0 {

				script += `echo "Setting up primary server 1"
`

				script += fmt.Sprintf(`k3sup install --host %s \
--user %s \
--cluster \
--local-path %s \
--context %s%s%s%s%s
`,
					host.IP,
					user,
					kubeconfig,
					contextName,
					tlsSanStr,
					serverExtraArgsSt,
					sshKeySt,
					mergeStr)

				script += fmt.Sprintf(`
echo "Fetching the server's node-token into memory"

export NODE_TOKEN=$(k3sup node-token --host %s --user %s%s)
`, host.IP, user, sshKeySt)

				serversAdded = 1
				primaryServer = host
			} else if serversAdded < servers {
				script += fmt.Sprintf("\necho \"Setting up additional server: %d\"\n", serversAdded+1)

				script += fmt.Sprintf(`k3sup join \
--host %s \
--server-host %s \
--server \
--node-token "$NODE_TOKEN" \
--user %s%s%s%s%s
`, host.IP, primaryServer.IP, user, tlsSanStr, serverExtraArgsSt, sshKeySt, bgStr)

				serversAdded++
			} else {
				script += fmt.Sprintf("\necho \"Setting up worker: %d\"\n", (i+1)-serversAdded)

				script += fmt.Sprintf(`k3sup join \
--host %s \
--server-host %s \
--node-token "$NODE_TOKEN" \
--user %s%s%s%s
`, host.IP, primaryServer.IP, user, agentExtraArgsSt, sshKeySt, bgStr)
			}

			if nodeLimit > 0 && i+1 >= nodeLimit {
				break
			}
		}

		fmt.Printf("%s\n", script)

		return nil
	}

	return command
}

type Host struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}
