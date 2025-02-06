package cmd

import (
	"fmt"
	"net"
	"os"
	"path"
	"runtime"
	"strings"

	"errors"

	"github.com/alexellis/k3sup/pkg"
	operator "github.com/alexellis/k3sup/pkg/operator"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

// MakeJoin creates the join command
func MakeJoin() *cobra.Command {
	var command = &cobra.Command{
		Use:   "join",
		Short: "Install the k3s agent on a remote host and join it to an existing server",
		Long: `Install the k3s agent on a remote host and join it to an existing server

` + pkg.SupportMessageShort + `
`,
		Example: `  # Install K3s joining a cluster as an agent
  k3sup join \
    --user AGENT_USER \
    --ip AGENT_IP \
    --server-ip IP \
    --server-user SERVER_USER

  # Install K3s joining a cluster as another server
  k3sup join \
    --user AGENT_USER \
    --ip AGENT_IP \
    --server \
    --server-ip IP \
    --server-user SERVER_USER

  # Join whilst specifying a channel for the k3sup version
  k3sup join --user pi \
    --server-host HOST \
    --host HOST \
    --k3s-channel latest`,
		SilenceUsage: true,
	}

	command.Flags().IP("ip", net.ParseIP("127.0.0.1"), "Public IP of node on which to install agent")
	command.Flags().IP("server-ip", net.ParseIP("127.0.0.1"), "Public IP of an existing k3s server")

	command.Flags().String("host", "", "Public hostname of node on which to install agent")
	command.Flags().String("server-host", "", "Public hostname of an existing k3s server")
	command.Flags().String("server-url", "", "If different from server-ip or server-host, the URL of the server to join")

	command.Flags().String("user", "root", "Username for SSH login")
	command.Flags().String("server-user", "root", "Server username for SSH login (Default to --user)")

	command.Flags().String("ssh-key", "~/.ssh/id_rsa", "The ssh key to use for remote login")
	command.Flags().String("server-ssh-key", "", "The ssh key of an existing k3s server")

	command.Flags().Int("ssh-port", 22, "The port on which to connect for ssh")
	command.Flags().Int("server-ssh-port", 22, "The port on which to connect to server for ssh (Default to --ssh-port)")
	command.Flags().Bool("skip-install", false, "Skip the k3s installer")
	command.Flags().Bool("sudo", true, "Use sudo for installation. e.g. set to false when using the root user and no sudo is available.")

	command.Flags().Bool("server", false, "Join the cluster as a server rather than as an agent for the embedded etcd mode")
	command.Flags().Bool("no-extras", false, `Disable "servicelb" and "traefik", when using --server flag`)
	command.Flags().Bool("print-command", false, "Print a command that you can use with SSH to manually recover from an error")
	command.Flags().String("node-token-path", "", "file containing --node-token")
	command.Flags().String("node-token", "", "prefetched token used by nodes to join the cluster")

	command.Flags().String("k3s-extra-args", "", "Additional arguments to pass to k3s installer, wrapped in quotes (e.g. --k3s-extra-args '--node-taint key=value:NoExecute')")
	command.Flags().String("k3s-version", "", "Set a version to install, overrides k3s-channel")
	command.Flags().String("k3s-channel", PinnedK3sChannel, "Release channel: stable, latest, or i.e. v1.19")

	command.Flags().String("tls-san", "", "Use an additional IP or hostname for the API server, when using --server flag")

	command.Flags().String("server-data-dir", "/var/lib/rancher/k3s/", "Override the path used to fetch the node-token from the server")

	command.RunE = func(command *cobra.Command, args []string) error {
		fmt.Printf("Running: k3sup join\n")

		ip, err := command.Flags().GetIP("ip")
		if err != nil {
			return err
		}

		var nodeToken string

		if command.Flags().Changed("node-token") {
			nodeToken, _ = command.Flags().GetString("node-token")
		} else if command.Flags().Changed("node-token-path") {
			nodeTokenPath, _ := command.Flags().GetString("node-token-path")
			if len(nodeTokenPath) > 0 {
				data, err := os.ReadFile(nodeTokenPath)
				if err != nil {
					return err
				}

				nodeToken = strings.TrimSpace(string(data))
			}
		}

		host, err := command.Flags().GetString("host")
		if err != nil {
			return err
		}
		if len(host) == 0 {
			host = ip.String()
		}

		dataDir, err := command.Flags().GetString("server-data-dir")
		if err != nil {
			return err
		}

		if len(dataDir) == 0 {
			return fmt.Errorf("--server-data-dir must be set")
		}

		if !strings.HasPrefix(dataDir, "/") {
			return fmt.Errorf("--server-data-dir must begin with /")
		}

		serverIP, err := command.Flags().GetIP("server-ip")
		if err != nil {
			return err
		}

		serverHost, err := command.Flags().GetString("server-host")
		if err != nil {
			return err
		}
		if len(serverHost) == 0 {
			serverHost = serverIP.String()
		}

		serverURL, err := command.Flags().GetString("server-url")
		if err != nil {
			return err
		}

		fmt.Printf("Joining %s => %s\n", host, serverHost)
		if len(serverURL) > 0 {
			fmt.Printf("Server join URL: %s\n", serverURL)
		}

		user, _ := command.Flags().GetString("user")

		serverUser := user
		if command.Flags().Changed("server-user") {
			serverUser, _ = command.Flags().GetString("server-user")
		}

		sshKey, _ := command.Flags().GetString("ssh-key")
		serverSshKey, _ := command.Flags().GetString("server-ssh-key")
		if len(serverSshKey) == 0 {
			serverSshKey = sshKey
		}

		server, err := command.Flags().GetBool("server")
		if err != nil {
			return err
		}

		port, _ := command.Flags().GetInt("ssh-port")
		serverPort := port
		if command.Flags().Changed("server-ssh-port") {
			serverPort, _ = command.Flags().GetInt("server-ssh-port")
		}

		k3sVersion, err := command.Flags().GetString("k3s-version")
		if err != nil {
			return err
		}
		k3sExtraArgs, err := command.Flags().GetString("k3s-extra-args")
		if err != nil {
			return err
		}
		k3sChannel, err := command.Flags().GetString("k3s-channel")
		if err != nil {
			return err
		}

		if len(k3sVersion) == 0 && len(k3sChannel) == 0 {
			return fmt.Errorf("give a value for --k3s-version or --k3s-channel")
		}

		printCommand, err := command.Flags().GetBool("print-command")
		if err != nil {
			return err
		}

		useSudo, err := command.Flags().GetBool("sudo")
		if err != nil {
			return err
		}
		sudoPrefix := ""
		if useSudo {
			sudoPrefix = "sudo "
		}
		sshKeyPath := expandPath(sshKey)
		serverSshKeyPath := expandPath(serverSshKey)

		if len(nodeToken) == 0 {
			address := fmt.Sprintf("%s:%d", serverHost, serverPort)

			sshOperator, sshOperatorDone, errored, err := connectOperator(serverUser, address, serverSshKeyPath)
			if errored {
				return err
			}

			if sshOperatorDone != nil {
				defer sshOperatorDone()
			}

			getTokenCommand := fmt.Sprintf("%scat %s\n", sudoPrefix, path.Join(dataDir, "/server/node-token"))
			if printCommand {
				fmt.Printf("ssh: %s\n", getTokenCommand)
			}

			streamToStdio := false
			res, err := sshOperator.ExecuteStdio(getTokenCommand, streamToStdio)

			if err != nil {
				return fmt.Errorf("unable to get join-token from server: %w", err)
			}

			if len(res.StdErr) > 0 {
				fmt.Printf("Error or warning getting node-token: %s\n", res.StdErr)
			} else {
				fmt.Printf("Received node-token from %s.. ok.\n", serverHost)
			}

			// Explicit close of the SSH connection as early as possible
			// which complements the defer
			if sshOperatorDone != nil {
				sshOperatorDone()
			}

			nodeToken = strings.TrimSpace(string(res.StdOut))
		}

		if server {

			tlsSan, _ := command.Flags().GetString("tls-san")
			noExtras, _ := command.Flags().GetBool("no-extras")

			err = setupAdditionalServer(serverHost, host, port, user, sshKeyPath, nodeToken, k3sExtraArgs, k3sVersion, k3sChannel, tlsSan, printCommand, serverURL, noExtras)
		} else {
			err = setupAgent(serverHost, host, port, user, sshKeyPath, nodeToken, k3sExtraArgs, k3sVersion, k3sChannel, printCommand, serverURL)
		}

		if err == nil {
			fmt.Printf("\n%s\n", pkg.SupportMessageShort)
		}

		return err
	}

	command.PreRunE = func(command *cobra.Command, args []string) error {

		_, err := command.Flags().GetIP("ip")
		if err != nil {
			return err
		}

		_, err = command.Flags().GetIP("server-ip")
		if err != nil {
			return err
		}

		_, err = command.Flags().GetString("host")
		if err != nil {
			return err
		}

		_, err = command.Flags().GetString("server-host")
		if err != nil {
			return err
		}

		_, err = command.Flags().GetInt("ssh-port")
		if err != nil {
			return err
		}

		tlsSan, err := command.Flags().GetString("tls-san")
		if err != nil {
			return err
		}

		noExtras, err := command.Flags().GetBool("no-extras")
		if err != nil {
			return err
		}

		if len(tlsSan) > 0 || noExtras {
			server, err := command.Flags().GetBool("server")
			if err != nil {
				return err
			}

			if !server {
				if noExtras {
					return fmt.Errorf("--no-extras can only be used with --server")
				}
				return fmt.Errorf("--tls-san can only be used with --server")
			}

		}

		return nil
	}

	return command
}

func setupAdditionalServer(serverHost, host string, port int, user, sshKeyPath, joinToken, k3sExtraArgs, k3sVersion, k3sChannel, tlsSAN string, printCommand bool, serverURL string, noExtras bool) error {
	address := fmt.Sprintf("%s:%d", host, port)

	var sshOperator *operator.SSHOperator
	var initialSSHErr error
	if runtime.GOOS != "windows" {

		var sshAgentAuthMethod ssh.AuthMethod
		sshAgentAuthMethod, initialSSHErr = sshAgentOnly()
		if initialSSHErr == nil {
			// Try SSH agent without parsing key files, will succeed if the user
			// has already added a key to the SSH Agent, or if using a configured
			// smartcard
			config := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{sshAgentAuthMethod},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}

			sshOperator, initialSSHErr = operator.NewSSHOperator(address, config)
		}
	} else {
		initialSSHErr = errors.New("ssh-agent unsupported on windows")
	}

	// If the initial connection attempt fails fall through to the using
	// the supplied/default private key file
	if initialSSHErr != nil {
		publicKeyFileAuth, closeSSHAgent, err := loadPublickey(sshKeyPath)
		if err != nil {
			return fmt.Errorf("unable to load the ssh key with path %q: %w", sshKeyPath, err)
		}

		defer closeSSHAgent()

		config := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				publicKeyFileAuth,
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		sshOperator, err = operator.NewSSHOperator(address, config)

		if err != nil {
			return fmt.Errorf("unable to connect to %s over ssh as %s: %w", address, user, err)
		}
	}

	installStr := createVersionStr(k3sVersion, k3sChannel)
	serverAgent := true

	defer sshOperator.Close()

	if noExtras {
		k3sExtraArgs += " --disable servicelb"
		k3sExtraArgs += " --disable traefik"
	}

	installk3sExec := makeJoinExec(
		serverHost,
		strings.TrimSpace(joinToken),
		installStr,
		k3sExtraArgs,
		serverAgent,
		serverURL,
		tlsSAN,
	)

	installAgentServerCommand := fmt.Sprintf("%s | %s", getScript, installk3sExec)

	if printCommand {
		fmt.Printf("ssh: %s\n", installAgentServerCommand)
	}

	res, err := sshOperator.Execute(installAgentServerCommand)
	if err != nil {
		return fmt.Errorf("unable to setup agent: %w", err)
	}

	if len(res.StdErr) > 0 {
		fmt.Printf("Logs: %s", res.StdErr)
	}

	joinRes := string(res.StdOut)
	fmt.Printf("Output: %s", string(joinRes))

	return nil
}

func setupAgent(serverHost, host string, port int, user, sshKeyPath, joinToken, k3sExtraArgs, k3sVersion, k3sChannel string, printCommand bool, serverURL string) error {

	address := fmt.Sprintf("%s:%d", host, port)

	var sshOperator *operator.SSHOperator
	var initialSSHErr error
	if runtime.GOOS != "windows" {

		var sshAgentAuthMethod ssh.AuthMethod
		sshAgentAuthMethod, initialSSHErr = sshAgentOnly()
		if initialSSHErr == nil {
			// Try SSH agent without parsing key files, will succeed if the user
			// has already added a key to the SSH Agent, or if using a configured
			// smartcard
			config := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{sshAgentAuthMethod},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}

			sshOperator, initialSSHErr = operator.NewSSHOperator(address, config)
		}
	} else {
		initialSSHErr = errors.New("ssh-agent unsupported on windows")
	}

	// If the initial connection attempt fails fall through to the using
	// the supplied/default private key file
	if initialSSHErr != nil {
		publicKeyFileAuth, closeSSHAgent, err := loadPublickey(sshKeyPath)
		if err != nil {
			return fmt.Errorf("unable to load the ssh key with path %q: %w", sshKeyPath, err)
		}

		defer closeSSHAgent()

		config := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				publicKeyFileAuth,
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		sshOperator, err = operator.NewSSHOperator(address, config)
		if err != nil {
			return fmt.Errorf("unable to connect to %s over ssh: %w", address, err)
		}
	}

	defer sshOperator.Close()

	installStr := createVersionStr(k3sVersion, k3sChannel)

	serverAgent := false

	// Agents don't expose an API server so don't need a TLS SAN
	tlsSAN := ""
	installK3sExec := makeJoinExec(
		serverHost,
		strings.TrimSpace(joinToken),
		installStr,
		k3sExtraArgs,
		serverAgent,
		serverURL,
		tlsSAN,
	)

	installAgentCommand := fmt.Sprintf("%s | %s", getScript, installK3sExec)

	if printCommand {
		fmt.Printf("ssh: %s\n", installAgentCommand)
	}

	res, err := sshOperator.Execute(installAgentCommand)

	if err != nil {
		return fmt.Errorf("unable to setup agent: %w", err)
	}

	if len(res.StdErr) > 0 {
		fmt.Printf("Logs: %s", res.StdErr)
	}

	joinRes := string(res.StdOut)
	fmt.Printf("Output: %s", string(joinRes))

	return nil
}

func createVersionStr(k3sVersion, k3sChannel string) string {
	installStr := ""
	if len(k3sVersion) > 0 {
		installStr = fmt.Sprintf("INSTALL_K3S_VERSION='%s'", k3sVersion)
	} else {
		installStr = fmt.Sprintf("INSTALL_K3S_CHANNEL='%s'", k3sChannel)
	}
	return installStr
}

func makeJoinExec(serverIP, joinToken, installStr, k3sExtraArgs string, serverAgent bool, serverURL, tlsSan string) string {

	installEnvVar := []string{}
	remoteURL := fmt.Sprintf("https://%s:6443", serverIP)
	if len(serverURL) > 0 {
		remoteURL = serverURL
	}

	installEnvVar = append(installEnvVar, fmt.Sprintf("K3S_URL='%s'", remoteURL))
	installEnvVar = append(installEnvVar, fmt.Sprintf("K3S_TOKEN='%s'", joinToken))
	installEnvVar = append(installEnvVar, installStr)

	if serverAgent {
		tlsSANValue := ""
		if len(tlsSan) > 0 {
			tlsSANValue = fmt.Sprintf(" --tls-san %s", tlsSan)
		}
		installEnvVar = append(installEnvVar, fmt.Sprintf("INSTALL_K3S_EXEC='server --server %s%s'", remoteURL, tlsSANValue))
	}

	joinExec := strings.Join(installEnvVar, " ")
	joinExec += " sh -s -"

	if len(k3sExtraArgs) > 0 {
		// AE: this doesn't seem to be used
		// installEnvVar = append(installEnvVar, k3sExtraArgs)

		joinExec += fmt.Sprintf(" %s", k3sExtraArgs)
	}

	return joinExec
}
