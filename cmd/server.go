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
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	"github.com/boxboat/dockhand-secrets-operator/pkg/k8s"
	"github.com/boxboat/dockhand-secrets-operator/pkg/webhook"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/leaderelection"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ServerArgs struct {
	serverPort       int
	serverCert       string
	serverKey        string
	serviceName      string
	serviceId        string
	serviceNamespace string
	selfSignCerts    bool
}

var (
	serverArgs ServerArgs
)

func certManager() {
	lock, err := k8s.GetLeaseLock(serverArgs.serviceId, serverArgs.serviceName, serverArgs.serviceNamespace)
	common.ExitIfError(err)

	leaderCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	leaderelection.RunOrDie(leaderCtx, leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				var cert *tls.Certificate
				checkTime := time.Now()
				for {
					// only retrieve the certificate at startup and then once a day after that
					if cert == nil || time.Now().Add(time.Hour*-24).After(checkTime) {
						common.Log.Infof("checking certificate %s/%s", serverArgs.serviceNamespace, serverArgs.serviceName)
						checkTime = time.Now()
						cert, err = k8s.GetServiceCertificate(leaderCtx, serverArgs.serviceName, serverArgs.serviceNamespace)
						if err != nil && !errors.IsNotFound(err) {
							common.ExitIfError(err)
						}
					}
					if cert == nil || common.ValidDaysRemaining(cert.Certificate[0]) < 30 {
						common.Log.Infof("Renewing self signed certificate")
						caPem, caKey, err := common.GenerateSelfSignedCA(serverArgs.serviceName + "-ca")
						common.ExitIfError(err)
						err = k8s.UpdateCABundleForWebhook(leaderCtx, serverArgs.serviceName+".dockhand.boxboat.io", caPem)
						common.ExitIfError(err)
						dnsNames := []string{
							serverArgs.serviceName + "." + serverArgs.serviceNamespace,
							serverArgs.serviceName + "." + serverArgs.serviceNamespace + ".svc"}

						cert, key, err := common.GenerateSignedCert(serverArgs.serviceName, dnsNames, caPem, caKey)
						common.ExitIfError(err)
						err = k8s.UpdateTlsCertificateSecret(leaderCtx, serverArgs.serviceName, serverArgs.serviceNamespace, cert, key, caPem)
						common.ExitIfError(err)

						webhook, err := k8s.GetDeployment(leaderCtx, serverArgs.serviceName, serverArgs.serviceNamespace)
						common.ExitIfError(err)
						checksum, err := k8s.GetSecretsChecksum(leaderCtx, []string{serverArgs.serviceName}, serverArgs.serviceNamespace)
						common.ExitIfError(err)
						if webhook.Spec.Template.Annotations == nil {
							webhook.Spec.Template.Annotations = make(map[string]string)
						}
						webhook.Spec.Template.Annotations[dockhand.SecretChecksumAnnotationKey] = checksum
						webhook, err = k8s.UpdateDeployment(leaderCtx, webhook, serverArgs.serviceNamespace)
						if err != nil {
							common.Log.Warnf("Could not update deployment %v", err)
						}
					}
					time.Sleep(time.Second * 5)
				}
			},
			OnStoppedLeading: func() {
				common.Log.Infof("No longer leading")
			},
			OnNewLeader: func(identity string) {
				if identity == serverArgs.serviceId {
					return
				}
			},
		},
		WatchDog:        nil,
		ReleaseOnCancel: true,
		Name:            serverArgs.serviceId,
	})

}

func runServer(ctx context.Context) {
	var err error
	tlsPair := tls.Certificate{}
	if serverArgs.selfSignCerts {
		go certManager()
	} else {
		tlsPair, err = tls.LoadX509KeyPair(serverArgs.serverCert, serverArgs.serverKey)
		common.ExitIfError(err)
	}

	attempt := 0
	for {
		if attempt < 10 {
			cert, err := k8s.GetServiceCertificate(ctx, serverArgs.serviceName, serverArgs.serviceNamespace)
			if errors.IsNotFound(err) {
				time.Sleep(5 * time.Second)
			} else if err != nil {
				common.Log.Warnf("error retrieving certificate, %v", err)
			} else {
				tlsPair = *cert
				break
			}
			attempt += 1
		}
	}

	common.Log.Infof("Starting server")

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

	startServerCmd.Flags().StringVar(
		&serverArgs.serviceId,
		"webhook-id",
		"",
		"webhook server id")

	startServerCmd.Flags().BoolVar(
		&serverArgs.selfSignCerts,
		"self-sign-certs",
		true,
		"use k8s api to obtain self signed certificates")

}
