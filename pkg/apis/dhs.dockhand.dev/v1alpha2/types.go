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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AutoUpdateLabelKey                            = "dhs.dockhand.dev/autoUpdate"
	DockhandSecretLabelKey                        = "dhs.dockhand.dev/ownedByDockhandSecret"
	DockhandSecretNamesLabelPrefixKey             = "secret.dhs.dockhand.dev/"
	SecretNamesAnnotationKey                      = "dhs.dockhand.dev/secretNames"
	SecretChecksumAnnotationKey                   = "dhs.dockhand.dev/secretChecksum"
	Ready                             SecretState = "Ready"
	Pending                           SecretState = "Pending"
	ErrApplied                        SecretState = "ErrApplied"
)

type SecretState string

// SecretRef specifies a reference to a Secret
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// AwsSecretsManager specifies the configuration for accessing AWS Secrets.
type AwsSecretsManager struct {
	CacheTTL           string     `json:"cacheTTL"`
	Region             string     `json:"region"`
	AccessKeyId        *string    `json:"accessKeyId,omitempty"`
	SecretAccessKeyRef *SecretRef `json:"secretAccessKeyRef,omitempty"`
}

// AzureKeyVault specifies the configuration for accessing Azure Key Vault secrets.
type AzureKeyVault struct {
	CacheTTL        string     `json:"cacheTTL"`
	Tenant          string     `json:"tenant"`
	ClientId        *string    `json:"clientId,omitempty"`
	ClientSecretRef *SecretRef `json:"clientSecretRef,omitempty"`
	KeyVault        string     `json:"keyVault"`
}

type GcpSecretsManager struct {
	CacheTTL                 string     `json:"cacheTTL"`
	Project                  string     `json:"project"`
	CredentialsFileSecretRef *SecretRef `json:"credentialsFileSecretRef"`
}

// Vault specifies the configuration for accessing Vault secrets.
type Vault struct {
	CacheTTL    string     `json:"cacheTTL"`
	Addr        string     `json:"addr"`
	RoleId      *string    `json:"roleId,omitempty"`
	SecretIdRef *SecretRef `json:"secretIdRef,omitempty"`
	TokenRef    *SecretRef `json:"tokenRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Profile is a specification for a DockhandProfile resource
type Profile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	AwsSecretsManager *AwsSecretsManager `json:"awsSecretsManager,omitempty"`
	AzureKeyVault     *AzureKeyVault     `json:"azureKeyVault,omitempty"`
	GcpSecretsManager *GcpSecretsManager `json:"gcpSecretsManager,omitempty"`
	Vault             *Vault             `json:"vault,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Secret is a specification for a Secret resource.
type Secret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	SyncInterval string            `json:"syncInterval"`
	Data         map[string]string `json:"data"`
	SecretSpec   SecretSpec        `json:"secretSpec"`
	Profile      ProfileRef        `json:"profile"`
	Status       SecretStatus      `json:"status,omitempty"`
}

type ProfileRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// SecretSpec defines the kubernetes secret data to use for the secret managed by a Secret
type SecretSpec struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type SecretStatus struct {
	State                         SecretState `json:"state"`
	ObservedAnnotationChecksum    string      `json:"observedAnnotationChecksum"`
	ObservedGeneration            int64       `json:"observedGeneration"`
	ObservedSecretResourceVersion string      `json:"observedSecretResourceVersion"`
	SyncTimestamp                 string      `json:"syncTimestamp"`
}
