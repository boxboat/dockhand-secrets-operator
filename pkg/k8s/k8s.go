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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	dockhandv1alpha1 "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	"github.com/gobuffalo/packr/v2/file/resolver/encoding/hex"
	certv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sort"
	"strings"
	"time"
)

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

const certRenewalTime = 30


func CopyStringMap(source map[string]string) map[string]string{

	copy := make(map[string]string)
	for k,v := range source {
		copy[k] = v
	}
	return copy
}

func GetDockhandSecretsListFromK8sSecrets(ctx context.Context, secretNames []string, namespace string) ([]string, error) {
	config, err := rest.InClusterConfig()
	common.HandleError(err)
	clientset, err := kubernetes.NewForConfig(config)
	common.HandleError(err)

	secretClient := clientset.CoreV1().Secrets(namespace)

	var dhSecrets []string
	for _, secretName := range secretNames {
		if secret, err := secretClient.Get(ctx, secretName, metav1.GetOptions{}); err == nil {
			if secret.Labels != nil {
				if val, ok := secret.Labels[dockhandv1alpha1.DockhandSecretLabelKey]; ok {
					dhSecrets = append(dhSecrets, val)
				}
			}
		}
	}
	common.Log.Infof("adding dockhand secrets[%s]", strings.Join(dhSecrets, ","))
	return dhSecrets, nil

}

// GetSecretsChecksum takes a set of secrets in a namespace and returns a checksum of all of the data in those secrets
func GetSecretsChecksum(ctx context.Context, names []string, namespace string) (string, error) {
	config, err := rest.InClusterConfig()
	common.HandleError(err)
	clientset, err := kubernetes.NewForConfig(config)
	common.HandleError(err)

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
		for k, _ := range secret.Data {
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
func UpdateCABundleForWebhook(ctx context.Context, name string, namespace string) {
	config, err := rest.InClusterConfig()
	common.HandleError(err)
	clientset, err := kubernetes.NewForConfig(config)
	common.HandleError(err)

	webhookClient := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations()
	webhook, err := webhookClient.Get(ctx, name, metav1.GetOptions{})
	common.HandleError(err)

	secretsClient := clientset.CoreV1().Secrets(namespace)
	secrets, err := secretsClient.List(ctx, metav1.ListOptions{})
	common.HandleError(err)
	for _, secret := range secrets.Items {
		if secret.Annotations["kubernetes.io/service-account.name"] == "default" {
			if len(webhook.Webhooks) > 0 && webhook.Webhooks[0].Name == name {
				webhook.Webhooks[0].ClientConfig.CABundle = secret.Data["ca.crt"]
				_, err := webhookClient.Update(context.Background(), webhook, metav1.UpdateOptions{})
				common.HandleError(err)
			}
			break
		}
	}
}

// GetServiceCertificate for service and namespace.
func GetServiceCertificate(ctx context.Context, name string, namespace string) tls.Certificate {
	config, err := rest.InClusterConfig()
	common.HandleError(err)
	clientset, err := kubernetes.NewForConfig(config)
	common.HandleError(err)

	nameDotNamespace := name + "." + namespace
	fullName := nameDotNamespace + ".svc"

	csrClient := clientset.CertificatesV1beta1().CertificateSigningRequests()

	existingCsr, err := csrClient.Get(ctx, nameDotNamespace, metav1.GetOptions{})

	if err == nil {
		if existingCsr.Status.Certificate != nil {
			common.Log.Infof("CSR[%s] exists", existingCsr.Name)
			block, _ := pem.Decode(existingCsr.Status.Certificate)
			cert, err := x509.ParseCertificate(block.Bytes)
			common.HandleError(err)

			validForDays := int(cert.NotAfter.Sub(time.Now()).Hours() / 24)
			common.Log.Infof("CSR[%s] status: valid for %d days", existingCsr.Name, validForDays)

			expired := validForDays <= certRenewalTime
			if expired {
				common.Log.Infof("CSR[%v] status: renewing", existingCsr.Name)
				err = csrClient.Delete(ctx, existingCsr.Name, metav1.DeleteOptions{})
			} else {
				secretClient := clientset.CoreV1().Secrets(namespace)
				certificateSecret, err := secretClient.Get(ctx, existingCsr.Name, metav1.GetOptions{})
				common.HandleError(err)
				if err == nil {
					common.Log.Infof("CSR[%s] status: returning existing certificate", existingCsr.Name)
					keyPair, err := tls.X509KeyPair(certificateSecret.Data["tls.crt"], certificateSecret.Data["tls.key"])
					common.HandleError(err)
					return keyPair
				}
			}
		} else {
			err = csrClient.Delete(ctx, existingCsr.Name, metav1.DeleteOptions{})
		}
	}

	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	common.HandleError(err)

	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: nameDotNamespace,
		},
		DNSNames: []string{name, nameDotNamespace, fullName},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, serverPrivateKey)
	common.HandleError(err)

	clientCSRPem := new(bytes.Buffer)
	_ = pem.Encode(
		clientCSRPem,
		&pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: csrBytes,
		})

	newCsr := &certv1beta1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: nameDotNamespace,
		},
		Spec: certv1beta1.CertificateSigningRequestSpec{
			Request: clientCSRPem.Bytes(),
			Usages:  []certv1beta1.KeyUsage{certv1beta1.UsageDigitalSignature, certv1beta1.UsageKeyEncipherment, certv1beta1.UsageServerAuth},
			Groups:  []string{"system:authenticated"},
		},
	}

	csr, err := csrClient.Create(ctx, newCsr, metav1.CreateOptions{})
	common.HandleError(err)
	csr.Status.Conditions = append(csr.Status.Conditions, certv1beta1.CertificateSigningRequestCondition{
		Type:           certv1beta1.CertificateApproved,
		Message:        "CSR approved by envoy-spire-mutating-webhook",
		LastUpdateTime: metav1.Now(),
	})

	_, err = csrClient.UpdateApproval(ctx, csr, metav1.UpdateOptions{})
	common.Log.Infof("CSR[%s] status: updated", csr.Name)

	attempt := 0
	for {
		if attempt < 5 {
			res, err := csrClient.Get(ctx, csr.Name, metav1.GetOptions{})
			common.HandleError(err)
			if res.Status.Certificate != nil {
				csr = res
				common.Log.Infof("CSR[%s] status: certificate issued", csr.Name)
				break
			}
			common.Log.Infof("CSR[%s] status: not issued yet retrying", csr.Name)
			time.Sleep(1 * time.Second)
		} else {
			common.HandleError(fmt.Errorf("CSR[%v] not found backed off after 5th attempt", csr.Name))
		}
		attempt += 1
	}
	serverPrivateKeyPEM := new(bytes.Buffer)
	_ = pem.Encode(serverPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivateKey),
	})

	serverCert := csr.Status.Certificate

	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: csr.Name,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.key": serverPrivateKeyPEM.Bytes(),
			"tls.crt": serverCert,
		},
	}

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, tlsSecret, metav1.UpdateOptions{})
	if err != nil {
		_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, tlsSecret, metav1.CreateOptions{})
		common.HandleError(err)
		common.Log.Infof("Created secret[%s]", tlsSecret.Name)
	} else {
		common.Log.Infof("Updated secret[%s]", tlsSecret.Name)
	}

	keyPair, err := tls.X509KeyPair(serverCert, serverPrivateKeyPEM.Bytes())
	common.HandleError(err)

	return keyPair
}

func GenerateMetadataLabelsPatch(target map[string]string, added map[string]string) (patch []PatchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
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
	for key, _ := range target {
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
		if target == nil || target[key] == "" {
			target = map[string]string{}
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
	for key, _ := range target {
		if added == nil || added[key] == "" {
			patch = append(patch, PatchOperation{
				Op:   "remove",
				Path: "/spec/template/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
			})
		}
	}
	return patch
}
