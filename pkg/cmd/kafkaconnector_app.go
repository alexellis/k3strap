package cmd

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/alexellis/k3sup/pkg/config"

	"github.com/spf13/cobra"
)

func makeInstallKafkaConnector() *cobra.Command {
	var command = &cobra.Command{
		Use:          "kafka-connector",
		Short:        "Install kafka-connector for OpenFaaS",
		Long:         `Install kafka-connector for OpenFaaS`,
		Example:      `  k3sup app install kafka-connector`,
		SilenceUsage: true,
	}

	command.Flags().StringP("namespace", "n", "openfaas", "The namespace used for installation")
	command.Flags().Bool("update-repo", true, "Update the helm repo")
	command.Flags().StringP("topics", "t", "faas-request", "The topics for the connector to bind to")
	command.Flags().String("broker-host", "kafka", "The host for the Kafka broker")

	command.RunE = func(command *cobra.Command, args []string) error {
		kubeConfigPath := getDefaultKubeconfig()

		if command.Flags().Changed("kubeconfig") {
			kubeConfigPath, _ = command.Flags().GetString("kubeconfig")
		}

		updateRepo, _ := command.Flags().GetBool("update-repo")

		fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)

		userPath, err := config.InitUserDir()
		if err != nil {
			return err
		}

		namespace, _ := command.Flags().GetString("namespace")

		if namespace != "openfaas" {
			return fmt.Errorf(`to override the "openfaas", install via tiller`)
		}

		clientArch, clientOS := getClientArch()

		fmt.Printf("Client: %s, %s\n", clientArch, clientOS)
		log.Printf("User dir established as: %s\n", userPath)

		os.Setenv("HELM_HOME", path.Join(userPath, ".helm"))

		_, err = tryDownloadHelm(userPath, clientArch, clientOS)
		if err != nil {
			return err
		}

		err = addHelmRepo("openfaas", "https://openfaas.github.io/faas-netes/")
		if err != nil {
			return err
		}

		if updateRepo {
			err = updateHelmRepos()
			if err != nil {
				return err
			}
		}

		chartPath := path.Join(os.TempDir(), "charts")
		err = fetchChart(chartPath, "openfaas/kafka-connector")

		if err != nil {
			return err
		}

		topicsVal, err := command.Flags().GetString("topics")
		if err != nil {
			return err
		}

		brokerHostVal, err := command.Flags().GetString("broker-host")
		if err != nil {
			return err
		}

		overrides := map[string]string{
			"topics":      topicsVal,
			"broker_host": brokerHostVal,
		}

		arch := getArchitecture()
		fmt.Printf("Node architecture: %q\n", arch)

		fmt.Println("Chart path: ", chartPath)

		outputPath := path.Join(chartPath, "kafka-connector/rendered")

		ns := namespace
		err = templateChart(chartPath,
			"kafka-connector",
			ns,
			outputPath,
			"values.yaml",
			overrides)

		if err != nil {
			return err
		}

		err = kubectl("apply", "-R", "-f", outputPath)

		if err != nil {
			return err
		}

		fmt.Println(`=======================================================================
= kafka-connector has been installed.                                   =
=======================================================================

# View the connector's logs:

kubectl logs deploy/kafka-connector -n openfaas -f

# Find out more on the project homepage:

# https://github.com/openfaas-incubator/kafka-connector/

` + thanksForUsing)

		return nil
	}

	return command
}
