package cmd

import (
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/alexellis/k3sup/pkg"
	ssh "github.com/alexellis/k3sup/pkg/operator"

	"github.com/spf13/cobra"
)

// MakeNodeToken creates the node-token command
func MakeNodeToken() *cobra.Command {
	var command = &cobra.Command{
		Use:   "node-token",
		Short: "Retrieve the node token from a server",
		Long: `Retrieve the node token from a server required for a
server or agent to join the cluster.

` + pkg.SupportMessageShort + `
`,
		Example: `  # Get the node token from the server and pipe it to a file
  k3sup node-token --ip IP --user USER > token.txt
`,
		SilenceUsage: true,
	}

	command.Flags().IP("ip", net.ParseIP("127.0.0.1"), "Public IP of node")
	command.Flags().String("user", "root", "Username for SSH login")

	command.Flags().String("host", "", "Public hostname of node on which to install agent")

	command.Flags().Bool("local", false, "Use local machine instead of ssh client")
	command.Flags().String("ssh-key", "~/.ssh/id_rsa", "The ssh key to use for remote login")
	command.Flags().Int("ssh-port", 22, "The port on which to connect for ssh")
	command.Flags().Bool("sudo", true, "Use sudo for installation. e.g. set to false when using the root user and no sudo is available.")

	command.Flags().Bool("print-command", false, "Print the command to be executed")
	command.Flags().String("server-data-dir", "/var/lib/rancher/k3s/", "Override the path used to fetch the node-token from the server")

	command.PreRunE = func(command *cobra.Command, args []string) error {
		local, err := command.Flags().GetBool("local")
		if err != nil {
			return err
		}

		if !local {
			_, err = command.Flags().GetString("host")
			if err != nil {
				return err
			}

			if _, err := command.Flags().GetIP("ip"); err != nil {
				return err
			}

			if _, err := command.Flags().GetInt("ssh-port"); err != nil {
				return err
			}
		}
		return nil
	}

	command.RunE = func(command *cobra.Command, args []string) error {

		fmt.Fprintf(os.Stderr, "Fetching: /etc/rancher/k3s/k3s.yaml\n")

		useSudo, err := command.Flags().GetBool("sudo")
		if err != nil {
			return err
		}

		sudoPrefix := ""
		if useSudo {
			sudoPrefix = "sudo "
		}

		local, _ := command.Flags().GetBool("local")

		ip, err := command.Flags().GetIP("ip")
		if err != nil {
			return err
		}

		host, err := command.Flags().GetString("host")
		if err != nil {
			return err
		}
		if len(host) == 0 {
			host = ip.String()
		}

		port, _ := command.Flags().GetInt("ssh-port")
		user, _ := command.Flags().GetString("user")
		sshKey, _ := command.Flags().GetString("ssh-key")

		dataDir, _ := command.Flags().GetString("server-data-dir")

		sshKeyPath := expandPath(sshKey)
		address := net.JoinHostPort(host, strconv.Itoa(port))
		if !local {
			fmt.Fprintf(os.Stderr, "Remote: %s\n", address)
		}

		printCommand := false

		getTokenCommand := fmt.Sprintf("%scat %s\n", sudoPrefix, path.Join(dataDir, "/server/node-token"))
		if printCommand {
			fmt.Printf("ssh: %s\n", getTokenCommand)
		}

		var operator ssh.CommandOperator
		if local {
			operator = ssh.ExecOperator{}
		} else {
			sshOperator, sshOperatorDone, errored, err := connectOperator(user, address, sshKeyPath)
			if errored {
				return err
			}
			operator = sshOperator

			if sshOperatorDone != nil {
				defer sshOperatorDone()
			}
		}

		nodeToken, err := obtainNodeToken(operator, getTokenCommand, host)
		if err != nil {
			return err
		}

		if len(nodeToken) == 0 {
			return fmt.Errorf("no node token found")
		}

		fmt.Println(nodeToken)
		return nil
	}

	return command
}

func obtainNodeToken(operator ssh.CommandOperator, command, host string) (string, error) {
	res, err := operator.ExecuteStdio(command, false)
	if err != nil {
		return "", fmt.Errorf("error received processing command: %s", err)
	}

	return strings.TrimSpace(string(res.StdOut)), nil

}
