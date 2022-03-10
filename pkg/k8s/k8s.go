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

package k8s

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"sort"
	"strings"

	dockhandv2 "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dhs.dockhand.dev/v1alpha2"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	"github.com/gobuffalo/packr/v2/file/resolver/encoding/hex"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func CopyStringMap(source map[string]string) map[string]string {

	mapCopy := make(map[string]string)
	for k, v := range source {
		mapCopy[k] = v
	}
	return mapCopy
}

func GetDeployment(ctx context.Context, name string, namespace string) (*v1.Deployment, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

func UpdateDeployment(ctx context.Context, deployment *v1.Deployment, namespace string) (*v1.Deployment, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
}

func GetLeaseLock(leaseId string, leaseName string, namespace string) (*resourcelock.LeaseLock, error) {
	common.Log.Infof("%s requesting %s/%s LeaseLock", leaseId, namespace, leaseName)
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: namespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: leaseId,
		},
	}, nil
}

func GetDockhandSecretsListFromK8sSecrets(ctx context.Context, secretNames []string, namespace string) ([]string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	secretClient := clientset.CoreV1().Secrets(namespace)

	var dhSecrets []string
	for _, secretName := range secretNames {
		if secret, err := secretClient.Get(ctx, secretName, metav1.GetOptions{}); !errors.IsNotFound(err) {
			if secret.Labels != nil {
				if val, ok := secret.Labels[dockhandv2.DockhandSecretLabelKey]; ok {
					dhSecrets = append(dhSecrets, val)
				}
			}
		}
	}
	return dhSecrets, nil

}

// GetSecretsChecksum takes a set of secrets in a namespace and returns a checksum of all of the data in those secrets
func GetSecretsChecksum(ctx context.Context, names []string, namespace string) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	// sort the names to ensure the checksum doesn't change
	sort.Strings(names)

	hash := sha1.New()

	secretsClient := clientset.CoreV1().Secrets(namespace)
	for _, name := range names {
		secret, err := secretsClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			common.Log.Warnf("error retrieving %s/%s %v", namespace, name, err)
			return "", fmt.Errorf("unable to checksum secret %s/%s", namespace, name)
		}
		var keys []string
		for k := range secret.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			hash.Write([]byte(k))
			hash.Write(secret.Data[k])
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// UpdateCABundleForWebhook updates the CA Bundle
func UpdateCABundleForWebhook(ctx context.Context, name string, caBundleBytes []byte) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	webhookClient := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations()
	webhook, err := webhookClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	change := false

	for idx := range webhook.Webhooks {
		if bytes.Compare(webhook.Webhooks[idx].ClientConfig.CABundle, caBundleBytes) != 0 {
			common.Log.Infof("updating %s CABundle", webhook.Webhooks[idx].Name)
			webhook.Webhooks[idx].ClientConfig.CABundle = caBundleBytes
			change = true
		}
	}
	if change {
		_, err := webhookClient.Update(context.Background(), webhook, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		common.Log.Debugf("no change detected with CA pem - not updating webhook configuration")
	}

	return nil
}

func UpdateTlsCertificateSecret(ctx context.Context, name string, namespace string, serverPem []byte, serverKey []byte, caPem []byte) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.key": serverKey,
			"tls.crt": serverPem,
			"ca.crt":  caPem,
		},
	}
	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, tlsSecret, metav1.UpdateOptions{})
	if errors.IsNotFound(err) {
		if _, err = clientset.CoreV1().Secrets(namespace).Create(ctx, tlsSecret, metav1.CreateOptions{}); err != nil {
			return err
		}
		common.Log.Infof("Created secret[%s]", tlsSecret.Name)
	} else {
		common.Log.Infof("Updated secret[%s]", tlsSecret.Name)
	}
	return nil

}

// GetServiceCertificate for service and namespace.
func GetServiceCertificate(ctx context.Context, name string, namespace string) (*tls.Certificate, []byte, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	tlsSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	keyPair, err := tls.X509KeyPair(tlsSecret.Data["tls.crt"], tlsSecret.Data["tls.key"])
	if err != nil {
		return nil, nil, err
	}

	caCert := tlsSecret.Data["ca.crt"]

	return &keyPair, caCert, nil
}

func GenerateMetadataLabelsPatch(target map[string]string, added map[string]string) (patch []PatchOperation) {
	for key, value := range added {
		if target == nil {
			target = map[string]string{}
			target[key] = value
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/metadata/labels",
				Value: target,
			})
		} else if target[key] == "" {
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/metadata/labels/" + strings.ReplaceAll(key, "/", "~1"),
				Value: value,
			})
		} else {
			patch = append(patch, PatchOperation{
				Op:    "replace",
				Path:  "/metadata/labels/" + strings.ReplaceAll(key, "/", "~1"),
				Value: value,
			})
		}
	}
	for key := range target {
		if added == nil || added[key] == "" {
			patch = append(patch, PatchOperation{
				Op:   "remove",
				Path: "/metadata/labels/" + strings.ReplaceAll(key, "/", "~1"),
			})
		}
	}
	return patch
}

func GenerateSpecTemplateAnnotationPatch(target map[string]string, added map[string]string) (patch []PatchOperation) {

	for key, value := range added {
		if target == nil {
			target = map[string]string{}
			target[key] = value
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/spec/template/metadata/annotations",
				Value: target,
			})
		} else if target[key] == "" {
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/spec/template/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
				Value: value,
			})
		} else {
			patch = append(patch, PatchOperation{
				Op:    "replace",
				Path:  "/spec/template/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
				Value: value,
			})
		}
	}
	for key := range target {
		if added == nil || added[key] == "" {
			patch = append(patch, PatchOperation{
				Op:   "remove",
				Path: "/spec/template/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
			})
		}
	}
	return patch
}
