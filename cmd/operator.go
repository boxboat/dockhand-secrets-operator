/*
Copyright © 2021 BoxBoat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"github.com/boxboat/dockcmd/cmd/aws"
	"github.com/boxboat/dockcmd/cmd/azure"
	dockcmdCommon "github.com/boxboat/dockcmd/cmd/common"
	"github.com/boxboat/dockcmd/cmd/gcp"
	"github.com/boxboat/dockcmd/cmd/vault"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	controllerv1 "github.com/boxboat/dockhand-secrets-operator/pkg/controller/v1"
	controllerv2 "github.com/boxboat/dockhand-secrets-operator/pkg/controller/v2"
	dockhandv2 "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dhs.dockhand.dev"
	dockhandv1 "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dockhand.boxboat.io"
	"github.com/rancher/wrangler/pkg/generated/controllers/apps"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"text/template"
)

type OperatorArgs struct {
	MasterURL      string
	KubeconfigFile string
	Namespace      string
	CrossNamespaceProfileAccessAuthorized bool
}

var (
	operatorArgs OperatorArgs
)

// awsRegionCmdPersistentPreRunE checks required persistent tokens
func startOperatorCmdPersistentPreRunE(cmd *cobra.Command, args []string) error {
	if err := rootCmdPersistentPreRunE(cmd, args); err != nil {
		return err
	}
	common.Log.Debugln("awsCmdPersistentPreRunE")
	aws.Region = viper.GetString("aws-region")
	common.Log.Debugf("Using AWS Region: {%s}", aws.Region)
	aws.AccessKeyID = viper.GetString("aws-access-key-id")
	aws.SecretAccessKey = viper.GetString("aws-secret-access-key")

	vault.Addr = viper.GetString("vault-addr")
	vault.Token = viper.GetString("vault-token")
	vault.RoleID = viper.GetString("vault-role-id")
	vault.SecretID = viper.GetString("vault-secret-id")

	if aws.AccessKeyID == "" && aws.SecretAccessKey == "" {
		aws.UseChainCredentials = true
	}

	if vault.RoleID != "" && vault.SecretID != "" {
		vault.Auth = vault.RoleAuth
	} else {
		vault.Auth = vault.TokenAuth
	}

	azure.TenantID = viper.GetString("azure-tenant")
	azure.ClientID = viper.GetString("azure-client-id")
	azure.ClientSecret = viper.GetString("azure-client-secret")

	return nil
}

var startOperatorCmd = &cobra.Command{
	Use:               "controller",
	Short:             "controller start",
	Long:              `start the operator controller with the provided settings`,
	PersistentPreRunE: startOperatorCmdPersistentPreRunE,
	Run: func(cmd *cobra.Command, args []string) {

		// load function maps
		funcMap := template.FuncMap{
			"aws":       aws.GetAwsSecret,
			"vault":     vault.GetVaultSecret,
			"azureJson": azure.GetAzureJSONSecret,
			"azureText": azure.GetAzureTextSecret,
			"gcpJson":   gcp.GetJSONSecret,
			"gcpText":   gcp.GetTextSecret,
		}
		dockcmdCommon.UseAlternateDelims = true

		// load the kubeconfig file
		cfg, err := kubeconfig.GetNonInteractiveClientConfig(
			operatorArgs.KubeconfigFile).ClientConfig()
		if err != nil {
			logrus.Fatalf("Error building kubeconfig: %s", err.Error())
		}

		// Generated controllers
		apps := apps.NewFactoryFromConfigOrDie(cfg)
		core := core.NewFactoryFromConfigOrDie(cfg)
		// TODO deprecated remove v1
		dhv1 := dockhandv1.NewFactoryFromConfigOrDie(cfg)
		dhv2 := dockhandv2.NewFactoryFromConfigOrDie(cfg)
		kubeClient := kubernetes.NewForConfigOrDie(cfg)

		v2handler := controllerv2.Register(
			cmd.Context(),
			operatorArgs.Namespace,
			kubeClient.CoreV1().Events(""),
			apps.Apps().V1().DaemonSet(),
			apps.Apps().V1().Deployment(),
			apps.Apps().V1().StatefulSet(),
			core.Core().V1().Secret(),
			dhv2.Dhs().V1alpha2().Secret(),
			dhv2.Dhs().V1alpha2().Profile(),
			funcMap,
			operatorArgs.CrossNamespaceProfileAccessAuthorized)

		// TODO deprecated remove v1
		controllerv1.Register(
			cmd.Context(),
			operatorArgs.Namespace,
			kubeClient.CoreV1().Events(""),
			core.Core().V1().Secret(),
			dhv1.Dockhand().V1alpha1().DockhandSecret(),
			dhv1.Dockhand().V1alpha1().DockhandSecretsProfile(),
			funcMap,
			v2handler)

		// Start all the controllers
		if err := start.All(cmd.Context(), 2, apps, dhv1, dhv2); err != nil {
			logrus.Fatalf("Error starting: %s", err.Error())
		}
		<-cmd.Context().Done()
	},
}

// setup command
func init() {
	rootCmd.AddCommand(startOperatorCmd)

	startOperatorCmd.PersistentFlags().StringVar(
		&operatorArgs.KubeconfigFile,
		"kubeconfig",
		"",
		"Path to a kubeconfig. Only required if out of cluster")

	startOperatorCmd.PersistentFlags().StringVar(
		&operatorArgs.MasterURL,
		"master",
		"",
		"Address of Kube API server. Overrides value in kubeconfig. Only required if out of cluster.")

	startOperatorCmd.PersistentFlags().StringVar(
		&operatorArgs.Namespace,
		"namespace",
		"",
		"Namespace where the operator is deployed.")

	startOperatorCmd.PersistentFlags().BoolVar(
		&operatorArgs.CrossNamespaceProfileAccessAuthorized,
		"allow-cross-namespace",
		false,
		"Allow Secrets to specify Profiles in external namespaces. i.e. Secret Alpha in namespace alpha could reference a profile in namespace Bravo")

	startOperatorCmd.PersistentFlags().StringVar(
		&aws.Region,
		"aws-region",
		"",
		"AWS Region can alternatively be set using ${AWS_DEFAULT_REGION}")
	_ = viper.BindEnv("aws-region", "AWS_DEFAULT_REGION")

	startOperatorCmd.PersistentFlags().StringVar(
		&aws.AccessKeyID,
		"access-key-id",
		"",
		"AWS Access Key ID can alternatively be set using ${AWS_ACCESS_KEY_ID}")
	_ = viper.BindEnv("aws-access-key-id", "AWS_ACCESS_KEY_ID")

	startOperatorCmd.PersistentFlags().StringVar(
		&aws.SecretAccessKey,
		"aws-secret-access-key",
		"",
		"AWS Secret Access Key can alternatively be set using ${AWS_SECRET_ACCESS_KEY}")
	_ = viper.BindEnv("aws-secret-access-key", "AWS_SECRET_ACCESS_KEY")

	startOperatorCmd.PersistentFlags().StringVar(
		&aws.Profile,
		"aws-profile",
		"",
		"AWS Profile can alternatively be set using ${AWS_PROFILE}")
	_ = viper.BindEnv("aws-region", "AWS_PROFILE")

	startOperatorCmd.PersistentFlags().StringVarP(
		&azure.TenantID,
		"azure-tenant",
		"",
		"",
		"Azure tenant ID can alternatively be set using ${AZURE_TENANT_ID}")
	_ = viper.BindEnv("tenant", "AZURE_TENANT_ID")

	startOperatorCmd.PersistentFlags().StringVarP(
		&azure.ClientID,
		"azure-client-id",
		"",
		"",
		"Azure Client ID can alternatively be set using ${AZURE_CLIENT_ID}")

	startOperatorCmd.PersistentFlags().StringVarP(
		&azure.ClientSecret,
		"azure-client-secret",
		"",
		"",
		"Azure Client Secret Key can alternatively be set using ${AZURE_CLIENT_SECRET}")

	startOperatorCmd.PersistentFlags().StringVarP(
		&azure.KeyVaultName,
		"azure-key-vault",
		"",
		"",
		"Azure Key Vault Name")

	_ = viper.BindEnv("azure-tenant", "AZURE_TENANT_ID")
	_ = viper.BindEnv("azure-client-id", "AZURE_CLIENT_ID")
	_ = viper.BindEnv("azure-client-secret", "AZURE_CLIENT_SECRET")

	startOperatorCmd.PersistentFlags().StringVarP(
		&vault.Addr,
		"vault-addr",
		"",
		"",
		"Vault ADDR")
	viper.BindEnv("vault-addr", "VAULT_ADDR")
	startOperatorCmd.PersistentFlags().StringVarP(
		&vault.Token,
		"vault-token",
		"",
		"",
		"Vault Token can alternatively be set using ${VAULT_TOKEN}")

	startOperatorCmd.PersistentFlags().StringVarP(
		&vault.RoleID,
		"vault-role-id",
		"",
		"",
		"Vault Role Id if not using vault-token can alternatively be set using ${VAULT_ROLE_ID} (also requires vault-secret-id)")

	startOperatorCmd.PersistentFlags().StringVarP(
		&vault.SecretID,
		"vault-secret-id",
		"",
		"",
		"Vault Secret Id if not using vault-token can alternatively be set using ${VAULT_SECRET_ID} (also requires vault-role-id)")

	_ = viper.BindEnv("vault-token", "VAULT_TOKEN")
	_ = viper.BindEnv("vault-role-id", "VAULT_ROLE_ID")
	_ = viper.BindEnv("vault-secret-id", "VAULT_SECRET_ID")

	_ = viper.BindPFlags(startOperatorCmd.PersistentFlags())
}
