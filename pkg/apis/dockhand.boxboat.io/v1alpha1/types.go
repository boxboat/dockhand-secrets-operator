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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AutoUpdateLabelKey                = "dockhand.boxboat.io/autoUpdate"
	DockhandSecretLabelKey            = "dockhand.boxboat.io/ownedByDockhandSecret"
	DockhandSecretNamesLabelPrefixKey = "dockhandsecret.boxboat.io/"
	SecretNamesAnnotationKey          = "dockhand.boxboat.io/secretNames"
	SecretChecksumAnnotationKey       = "dockhand.boxboat.io/secretChecksum"
)

// AwsSecretsManager specifices the configuration for accessing AWS Secrets.
type AwsSecretsManager struct {
	CacheTTL           int     `json:"cacheTTL"`
	Region             string  `json:"region"`
	AccessKeyId        *string `json:"accessKeyId,omitempty"`
	SecretAccessKeyRef *string `json:"secretAccessKeyRef,omitempty"`
}

// AzureKeyVault specifies the configuration for accessing Azure Key Vault secrets.
type AzureKeyVault struct {
	CacheTTL        int     `json:"cacheTTL"`
	Tenant          string  `json:"tenant"`
	ClientId        *string `json:"clientId,omitempty"`
	ClientSecretRef *string `json:"clientSecretRef,omitempty"`
	KeyVault        string  `json:"keyVault"`
}

type GcpSecretsManager struct {
	CacheTTL                 int     `json:"cacheTTL"`
	Project                  string  `json:"project"`
	CredentialsFileSecretRef *string `json:"credentialsFileSecretRef"`
}

// Vault specifies the configuration for accessing Vault secrets.
type Vault struct {
	CacheTTL    int     `json:"cacheTTL"`
	Addr        string  `json:"addr"`
	RoleId      *string `json:"roleId,omitempty"`
	SecretIdRef *string `json:"secretIdRef,omitempty"`
	TokenRef    *string `json:"tokenRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DockhandProfile is a specification for a DockhandProfile resource
type DockhandProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	AwsSecretsManager *AwsSecretsManager `json:"awsSecretsManager,omitempty"`
	AzureKeyVault     *AzureKeyVault     `json:"azureKeyVault,omitempty"`
	GcpSecretsManager *GcpSecretsManager `json:"gcpSecretsManager,omitempty"`
	Vault             *Vault             `json:"vault,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DockhandSecret is a specification for a DockhandSecret resource.
type DockhandSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Data            map[string]string `json:"data"`
	SecretName      string            `json:"secretName"`
	DockhandProfile string            `json:"dockhandProfile"`

	Status DockhandSecretStatus `json:"status"`
}

type DockhandSecretStatus struct {
}
