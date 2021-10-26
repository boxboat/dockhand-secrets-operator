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

// Package v1 deprecated. TODO remove in next major release
package v1

import (
	"context"
	"github.com/boxboat/dockcmd/cmd/aws"
	"github.com/boxboat/dockcmd/cmd/azure"
	dockcmdCommon "github.com/boxboat/dockcmd/cmd/common"
	"github.com/boxboat/dockcmd/cmd/gcp"
	"github.com/boxboat/dockcmd/cmd/vault"
	dockhandv2 "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dhs.dockhand.dev/v1alpha2"
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	v2 "github.com/boxboat/dockhand-secrets-operator/pkg/controller/v2"
	dockhandcontrollers "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dockhand.boxboat.io/v1alpha1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"os"
	"text/template"
	"time"
)

// Handler is the controller implementation for Secret Resources
type Handler struct {
	ctx                        context.Context
	operatorNamespace          string
	dhSecretsController        dockhandcontrollers.DockhandSecretController
	dhSecretsProfileController dockhandcontrollers.DockhandSecretsProfileController
	dhProfileCache             dockhandcontrollers.DockhandSecretsProfileCache
	secrets                    corecontrollers.SecretClient
	funcMap                    template.FuncMap
	recorder                   record.EventRecorder
	v2Handler                  *v2.Handler
}

func Register(
	ctx context.Context,
	namespace string,
	events typedcorev1.EventInterface,
	secrets corecontrollers.SecretClient,
	dockhandSecrets dockhandcontrollers.DockhandSecretController,
	dockhandProfile dockhandcontrollers.DockhandSecretsProfileController,
	funcMap template.FuncMap,
	v2Handler *v2.Handler) {

	h := &Handler{
		ctx:                        ctx,
		operatorNamespace:          namespace,
		dhSecretsController:        dockhandSecrets,
		dhSecretsProfileController: dockhandProfile,
		dhProfileCache:             dockhandProfile.Cache(),
		secrets:                    secrets,
		funcMap:                    funcMap,
		recorder:                   buildEventRecorder(events),
		v2Handler:                  v2Handler,
	}

	// Register handlers
	dockhandSecrets.OnChange(ctx, "dockhandsecret-onchange", h.onDockhandSecretChange)
	dockhandSecrets.OnRemove(ctx, "dockhandsecret-onremove", h.onDockhandSecretRemove)
}

func buildEventRecorder(events typedcorev1.EventInterface) record.EventRecorder {
	// Create event broadcaster
	// Add dockhand controller types to the default Kubernetes Scheme so Events can be
	// logged for dockhand controller types.
	utilruntime.Must(dockhand.AddToScheme(scheme.Scheme))
	common.Log.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(common.Log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: events})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "dockhand-secrets-operator"})
}

func (h *Handler) onDockhandSecretRemove(_ string, secret *dockhand.DockhandSecret) (*dockhand.DockhandSecret, error) {
	if secret == nil {
		return nil, nil
	}
	common.Log.Debugf("Secret remove: %v", secret)
	common.Log.Debugf("removing secret=%s from namespace=%s", secret.SecretSpec.Name, secret.Namespace)
	if err := h.secrets.Delete(secret.Namespace, secret.SecretSpec.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		common.Log.Warnf(
			"could not delete secret=%s from namespace=%s",
			secret.SecretSpec.Name,
			secret.Namespace)
		return nil, err
	}

	return nil, nil
}

func (h *Handler) onDockhandSecretChange(_ string, secret *dockhand.DockhandSecret) (*dockhand.DockhandSecret, error) {
	if secret == nil {
		return nil, nil
	}

	if secret.Status.State == "" {
		statusErr := h.updateDockhandSecretStatus(secret, dockhand.Pending)
		common.LogIfError(statusErr)
	}

	common.Log.Debugf("Secret change: %v", secret)
	profile, err := h.dhProfileCache.Get(h.operatorNamespace, secret.Profile)

	if err != nil {
		common.Log.Warnf("Could not get Profile[%s]", secret.Profile)
		h.recorder.Eventf(secret, corev1.EventTypeWarning, "ErrLoadingProfile", "Could not get Profile[%s]", secret.Profile)
		statusErr := h.updateDockhandSecretStatus(secret, dockhand.ErrApplied)
		common.LogIfError(statusErr)
		return nil, err
	}

	if err := h.loadDockhandSecretsProfile(profile); err != nil {
		h.recorder.Eventf(secret, corev1.EventTypeWarning, "ErrLoadingProfile", "Could not load Profile: %v", err)
		statusErr := h.updateDockhandSecretStatus(secret, dockhand.ErrApplied)
		common.LogIfError(statusErr)
		return nil, err
	}

	k8sSecret, err := h.secrets.Get(secret.Namespace, secret.SecretSpec.Name, metav1.GetOptions{})

	newSecret := false
	if errors.IsNotFound(err) {
		newSecret = true
		k8sSecret = &corev1.Secret{
			Type: corev1.SecretType(secret.SecretSpec.Type),
			ObjectMeta: metav1.ObjectMeta{
				Name:        secret.SecretSpec.Name,
				Namespace:   secret.Namespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			Data: make(map[string][]byte),
		}
	} else {
		if k8sSecret.Labels == nil {
			k8sSecret.Labels = make(map[string]string)
		}
		if k8sSecret.Annotations == nil {
			k8sSecret.Annotations = make(map[string]string)
		}
	}

	if secret.SecretSpec.Labels != nil {
		for k, v := range secret.SecretSpec.Labels {
			k8sSecret.Labels[k] = v
		}
	}

	if secret.SecretSpec.Annotations != nil {
		for k, v := range secret.SecretSpec.Annotations {
			k8sSecret.Annotations[k] = v
		}
	}

	// Store reference in Secret to owning Secret
	k8sSecret.Labels[dockhandv2.DockhandSecretLabelKey] = secret.SecretSpec.Name

	for k, v := range secret.Data {
		secretData, err := dockcmdCommon.ParseSecretsTemplate([]byte(v), h.funcMap)
		if err != nil {
			h.recorder.Eventf(secret, corev1.EventTypeWarning, "ErrParsingSecret", "Could not parse template %v", err)
			statusErr := h.updateDockhandSecretStatus(secret, dockhand.ErrApplied)
			common.LogIfError(statusErr)
			return nil, err
		}
		common.Log.Debugf("%s: %s", k, secretData)
		k8sSecret.Data[k] = secretData
	}

	if newSecret {
		if _, err := h.secrets.Create(k8sSecret); err == nil {
			h.recorder.Eventf(secret, corev1.EventTypeNormal, "Success", "Secret %s/%s created", secret.Namespace, secret.SecretSpec.Name)
		} else {
			h.recorder.Eventf(secret, corev1.EventTypeWarning, "Error", "Secret %s/%s not created", secret.Namespace, secret.SecretSpec.Name)
			statusErr := h.updateDockhandSecretStatus(secret, dockhand.ErrApplied)
			common.LogIfError(statusErr)
			return nil, err
		}
	} else {
		if _, err := h.secrets.Update(k8sSecret); err == nil {
			h.recorder.Eventf(secret, corev1.EventTypeNormal, "Success", "Secret %s/%s updated", secret.Namespace, secret.SecretSpec.Name)
		} else {
			h.recorder.Eventf(secret, corev1.EventTypeWarning, "Error", "Secret %s/%s not updated", secret.Namespace, secret.SecretSpec.Name)
			statusErr := h.updateDockhandSecretStatus(secret, dockhand.ErrApplied)
			common.LogIfError(statusErr)
			return nil, err
		}
	}

	// if we have made it here the secret is provisioned and ready
	if err := h.updateDockhandSecretStatus(secret, dockhand.Ready); err != nil {
		// log status update error but continue
		common.LogIfError(err)
	}

	h.v2Handler.UpdateStatefulSets(secret.SecretSpec.Name, secret.Namespace)
	h.v2Handler.UpdateDaemonSets(secret.SecretSpec.Name, secret.Namespace)
	h.v2Handler.UpdateDeployments(secret.SecretSpec.Name, secret.Namespace)


	return nil, nil
}

func (h *Handler) loadDockhandSecretsProfile(profile *dockhand.DockhandSecretsProfile) error {
	if profile.AwsSecretsManager != nil {
		var err error
		if aws.CacheTTL, err = time.ParseDuration(profile.AwsSecretsManager.CacheTTL); err != nil {
			return err
		}

		aws.Region = profile.AwsSecretsManager.Region
		if profile.AwsSecretsManager.AccessKeyId != nil {
			aws.AccessKeyID = *profile.AwsSecretsManager.AccessKeyId
		}
		if profile.AwsSecretsManager.SecretAccessKeyRef != nil {
			secretData, err := h.secrets.Get(h.operatorNamespace, profile.AwsSecretsManager.SecretAccessKeyRef.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if secretData != nil {
				aws.SecretAccessKey = string(secretData.Data[profile.AwsSecretsManager.SecretAccessKeyRef.Key])
			}
		}
		if aws.AccessKeyID != "" && aws.SecretAccessKey != "" {
			aws.UseChainCredentials = false
		}
	}

	if profile.AzureKeyVault != nil {
		var err error
		if azure.CacheTTL, err = time.ParseDuration(profile.AzureKeyVault.CacheTTL); err != nil {
			return err
		}
		azure.KeyVaultName = profile.AzureKeyVault.KeyVault
		azure.TenantID = profile.AzureKeyVault.Tenant

		if profile.AzureKeyVault.ClientId != nil {
			azure.ClientID = *profile.AzureKeyVault.ClientId
		}

		if profile.AzureKeyVault.ClientSecretRef != nil {
			secretData, err := h.secrets.Get(h.operatorNamespace, profile.AzureKeyVault.ClientSecretRef.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if secretData != nil {
				azure.ClientSecret = string(secretData.Data[profile.AzureKeyVault.ClientSecretRef.Key])
			}
		}
		_ = os.Setenv("AZURE_TENANT_ID", azure.TenantID)
		_ = os.Setenv("AZURE_CLIENT_ID", azure.ClientID)
		_ = os.Setenv("AZURE_CLIENT_SECRET", azure.ClientSecret)
	}

	if profile.GcpSecretsManager != nil {
		var err error
		if gcp.CacheTTL, err = time.ParseDuration(profile.GcpSecretsManager.CacheTTL); err != nil {
			return err
		}
		gcp.Project = profile.GcpSecretsManager.Project
		if profile.GcpSecretsManager.CredentialsFileSecretRef != nil {
			secretData, err := h.secrets.Get(h.operatorNamespace, profile.GcpSecretsManager.CredentialsFileSecretRef.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if secretData != nil {
				gcp.CredentialsJson = secretData.Data[profile.GcpSecretsManager.CredentialsFileSecretRef.Key]
			}
		}
	}

	if profile.Vault != nil {
		var err error
		if vault.CacheTTL, err = time.ParseDuration(profile.Vault.CacheTTL); err != nil {
			return err
		}
		vault.Addr = profile.Vault.Addr
		if profile.Vault.RoleId != nil {
			vault.RoleID = *profile.Vault.RoleId
		}
		if profile.Vault.SecretIdRef != nil {
			secretData, err := h.secrets.Get(h.operatorNamespace, profile.Vault.SecretIdRef.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if secretData != nil {
				vault.SecretID = string(secretData.Data["VAULT_SECRET_ID"])
			}
		}
		if profile.Vault.TokenRef != nil {
			secretData, _ := h.secrets.Get(h.operatorNamespace, profile.Vault.TokenRef.Key, metav1.GetOptions{})
			if secretData != nil {
				vault.Token = string(secretData.Data["VAULT_TOKEN"])
			}
		}
	}

	return nil
}

func (h *Handler) updateDockhandSecretStatus(secret *dockhand.DockhandSecret, state dockhand.SecretState) error {
	common.Log.Infof("Updating %s status", secret.Name)
	secretCopy := secret.DeepCopy()
	secretCopy.Status.State = state
	_, err := h.dhSecretsController.UpdateStatus(secretCopy)

	return err
}
