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
	"context"
	"crypto/tls"
	"fmt"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	"github.com/boxboat/dockhand-secrets-operator/pkg/k8s"
	"github.com/boxboat/dockhand-secrets-operator/pkg/webhook"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type ServerArgs struct {
	serverPort        int
	serverCert        string
	serverKey         string
	serviceName       string
	serviceNamespace  string
	selfSignCerts     bool
}

var (
	serverArgs ServerArgs
)

func runServer(ctx context.Context) {

	var err error
	tlsPair := tls.Certificate{}
	if serverArgs.selfSignCerts {
		tlsPair, err = k8s.GetServiceCertificate(ctx, serverArgs.serviceName, serverArgs.serviceNamespace)
		common.ExitIfError(err)
		if err := k8s.UpdateCABundleForWebhook(ctx, serverArgs.serviceName + ".dockhand.boxboat.io", serverArgs.serviceNamespace); err != nil {
			common.ExitIfError(err)
		}
	} else {
		tlsPair, err = tls.LoadX509KeyPair(serverArgs.serverCert, serverArgs.serverKey)
		common.ExitIfError(err)
	}

	server := &webhook.Server{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%v", serverArgs.serverPort),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsPair},
			},
		},
	}

	server.Init()

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", server.Serve)
	server.Server.Handler = mux

	go func() {
		if err := server.Server.ListenAndServeTLS("", ""); err != nil {
			common.ExitIfError(err)
		}
	}()

	// listen for shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	common.Log.Infof("received shutdown signal, shutting down webhook gracefully")
	if err := server.Server.Shutdown(context.Background()); err != nil {
		common.Log.Infof("webhook server shutdown: %v", err)
	}
}

var startServerCmd = &cobra.Command{
	Use:   "server",
	Short: "webhook server",
	Long:  `start the server with the provided settings`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer(cmd.Context())
	},
}

// setup command
func init() {
	rootCmd.AddCommand(startServerCmd)

	startServerCmd.Flags().StringVar(
		&serverArgs.serviceName,
		"name",
		"dockhand-secrets-operator-webhook",
		"kubernetes service name")

	startServerCmd.Flags().StringVar(
		&serverArgs.serviceNamespace,
		"namespace",
		"dockhand-secrets-operator",
		"kubernetes service namespace")

	startServerCmd.Flags().IntVar(
		&serverArgs.serverPort,
		"port",
		8443,
		"")

	startServerCmd.Flags().StringVar(
		&serverArgs.serverCert,
		"cert",
		"/tls/server.crt",
		"x509 server certificate")

	startServerCmd.Flags().StringVar(
		&serverArgs.serverKey,
		"key",
		"/tls/server.key",
		"x509 server certificate")

	startServerCmd.Flags().BoolVar(
		&serverArgs.selfSignCerts,
		"self-sign-certs",
		true,
		"use k8s api to obtain self signed certificates")

}
