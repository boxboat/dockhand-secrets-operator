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
	dockcmdCommon "github.com/boxboat/dockcmd/cmd/common"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	controllerv2 "github.com/boxboat/dockhand-secrets-operator/pkg/controller/v2"
	dockhandv2 "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dhs.dockhand.dev"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/apps"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/start"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
)

type OperatorArgs struct {
	MasterURL                             string
	KubeconfigFile                        string
	Namespace                             string
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
	return nil
}

var startOperatorCmd = &cobra.Command{
	Use:               "controller",
	Short:             "controller start",
	Long:              `start the operator controller with the provided settings`,
	PersistentPreRunE: startOperatorCmdPersistentPreRunE,
	Run: func(cmd *cobra.Command, args []string) {

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
		dhv2 := dockhandv2.NewFactoryFromConfigOrDie(cfg)
		kubeClient := kubernetes.NewForConfigOrDie(cfg)

		controllerv2.Register(
			cmd.Context(),
			operatorArgs.Namespace,
			kubeClient.CoreV1().Events(""),
			apps.Apps().V1().DaemonSet(),
			apps.Apps().V1().Deployment(),
			apps.Apps().V1().StatefulSet(),
			core.Core().V1().Secret(),
			dhv2.Dhs().V1alpha2().Secret(),
			dhv2.Dhs().V1alpha2().Profile(),
			operatorArgs.CrossNamespaceProfileAccessAuthorized)

		// Start all the controllers
		if err := start.All(cmd.Context(), 2, apps, core, dhv2); err != nil {
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

	_ = viper.BindPFlags(startOperatorCmd.PersistentFlags())
}
