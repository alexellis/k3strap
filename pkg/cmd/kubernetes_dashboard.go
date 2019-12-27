package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

func makeInstallKubernetesDashboard() *cobra.Command {
	var kubeDashboard = &cobra.Command{
		Use:          "kubernetes-dashboard",
		Short:        "Install kubernetes-dashboard",
		Long:         `Install kubernetes-dashboard`,
		Example:      `  k3sup app install kubernetes-dashboard`,
		SilenceUsage: true,
	}

	kubeDashboard.RunE = func(command *cobra.Command, args []string) error {
		kubeConfigPath, _ := command.Flags().GetString("kubeconfig")

		fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)

		arch := getNodeArchitecture()
		fmt.Printf("Node architecture: %q\n", arch)

		res, err := kubectl(kubeConfigPath, "", "apply", "-f",
			"https://raw.githubusercontent.com/kubernetes/dashboard/v2.0.0-beta6/aio/deploy/recommended.yaml").Execute()

		if err != nil {
			return err
		}

		if res.ExitCode != 0 {
			return fmt.Errorf("kubectl exit code %d, stderr: %s",
				res.ExitCode,
				res.Stderr)
		}
		res, err = kubectl(kubeConfigPath, "", "apply", "-",
			`apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-user
  namespace: kubernetes-dashboard`).Execute()
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("kubectl exit code %d, stderr: %s",
				res.ExitCode,
				res.Stderr)
		}
		res, err = kubectl(kubeConfigPath, "", "apply", "-",
			`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-user
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: admin-user
  namespace: kubernetes-dashboard`).Execute()

		if err != nil {
			return err
		}

		if res.ExitCode != 0 {
			return fmt.Errorf("kubectl exit code %d, stderr: %s",
				res.ExitCode,
				res.Stderr)
		}

		fmt.Println(`=======================================================================
= Kubernetes Dashboard has been installed.                                        =
=======================================================================

#To forward the dashboard to your local machine 
kubectl proxy

#To get your Token for logging in
kubectl -n kubernetes-dashboard describe secret $(kubectl -n kubernetes-dashboard get secret | grep default-token | awk '{print $1}')

# Once Proxying you can navigate to the below
http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#/login

` + thanksForUsing)

		return nil
	}

	return kubeDashboard
}
