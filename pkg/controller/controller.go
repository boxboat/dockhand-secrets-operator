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

package controller

import (
	"context"
	"encoding/json"
	"github.com/boxboat/dockcmd/cmd/aws"
	"github.com/boxboat/dockcmd/cmd/azure"
	dockcmdCommon "github.com/boxboat/dockcmd/cmd/common"
	"github.com/boxboat/dockcmd/cmd/gcp"
	"github.com/boxboat/dockcmd/cmd/vault"
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	dockhandcontrollers "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/k8s"
	appscontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sort"
	"strings"
	"text/template"
)

// Handler is the controller implementation for DockhandSecret Resources
type Handler struct {
	ctx                       context.Context
	operatorNamespace         string
	daemonSets                appscontrollers.DaemonSetClient
	deployments               appscontrollers.DeploymentClient
	dockhandSecretsController dockhandcontrollers.DockhandSecretController
	dockhandProfileController dockhandcontrollers.DockhandProfileController
	dockhandProfileCache      dockhandcontrollers.DockhandProfileCache
	statefulSets              appscontrollers.StatefulSetClient
	secrets                   corecontrollers.SecretClient
	funcMap                   template.FuncMap
}

func Register(
	ctx context.Context,
	namespace string,
	daemonsets appscontrollers.DaemonSetController,
	deployments appscontrollers.DeploymentController,
	statefulsets appscontrollers.StatefulSetController,
	secrets corecontrollers.SecretClient,
	dockhandSecrets dockhandcontrollers.DockhandSecretController,
	dockhandProfile dockhandcontrollers.DockhandProfileController,
	funcMap template.FuncMap) {

	h := &Handler{
		ctx:                       ctx,
		operatorNamespace:         namespace,
		daemonSets:                daemonsets,
		deployments:               deployments,
		dockhandSecretsController: dockhandSecrets,
		dockhandProfileController: dockhandProfile,
		dockhandProfileCache:      dockhandProfile.Cache(),
		secrets:                   secrets,
		statefulSets:              statefulsets,
		funcMap:                   funcMap,
	}

	// Register handlers
	dockhandSecrets.OnChange(ctx, "dockhandsecret-onchange", h.onDockhandSecretChange)
	dockhandSecrets.OnRemove(ctx, "dockhandsecret-onremove", h.onDockhandSecretRemove)
	daemonsets.OnChange(ctx, "daemonsets-handler", h.onDaemonSetChange)
	deployments.OnChange(ctx, "deployment-handler", h.onDeploymentChange)
	statefulsets.OnChange(ctx, "daemonsets-handler", h.onStatefulSetChange)

}

func (h *Handler) onDockhandSecretRemove(_ string, secret *dockhand.DockhandSecret) (*dockhand.DockhandSecret, error) {
	if secret == nil {
		return nil, nil
	}
	common.Log.Debugf("DockhandSecret remove: %v", secret)
	common.Log.Debugf("removing secret=%s from namespace=%s", secret.SecretSpec.Name, secret.Namespace)
	if err := h.secrets.Delete(secret.Namespace, secret.SecretSpec.Name, &metav1.DeleteOptions{}); err != nil {
		common.Log.Warnf(
			"could not delete secret=%s from namespace=%s",
			secret.SecretSpec.Name,
			secret.Namespace)
	}

	return nil, nil
}

func (h *Handler) onDockhandSecretChange(_ string, secret *dockhand.DockhandSecret) (*dockhand.DockhandSecret, error) {
	if secret == nil {
		return nil, nil
	}
	common.Log.Debugf("DockhandSecret change: %v", secret)
	profile, err := h.dockhandProfileCache.Get(h.operatorNamespace, secret.DockhandProfile)
	if err != nil {
		common.Log.Warnf("Could not load DockhandProfile[%s]", secret.DockhandProfile)
		return nil, nil
	}
	h.loadDockhandProfile(profile)

	k8sSecret, err := h.secrets.Get(secret.Namespace, secret.SecretSpec.Name, metav1.GetOptions{})

	newSecret := false
	if err != nil {
		newSecret = true
		k8sSecret = &corev1.Secret{
			Type: corev1.SecretType(secret.SecretSpec.Type),
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.SecretSpec.Name,
				Namespace: secret.Namespace,
				Labels:    make(map[string]string),
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
		for k,v := range secret.SecretSpec.Labels{
			k8sSecret.Labels[k] = v
		}
	}

	if secret.SecretSpec.Annotations != nil {
		for k,v := range secret.SecretSpec.Annotations{
			k8sSecret.Annotations[k] = v
		}
	}

	// Store reference in Secret to owning DockhandSecret
	k8sSecret.Labels[dockhand.DockhandSecretLabelKey] = secret.SecretSpec.Name

	for k, v := range secret.Data {
		secretData, err := dockcmdCommon.ParseSecretsTemplate([]byte(v), h.funcMap)
		if err != nil {
			///TODO update dockhand secret status and try secret again later
		}
		common.Log.Debugf("%s: %s", k, secretData)
		k8sSecret.Data[k] = secretData
	}
	if newSecret {
		h.secrets.Create(k8sSecret)
	} else {
		h.secrets.Update(k8sSecret)
	}

	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + secret.SecretSpec.Name

	daemonsets, err := h.daemonSets.List(secret.Namespace, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
		return nil, nil
	}
	for _, daemonset := range daemonsets.Items {
		if _, err := h.processDaemonSet(&daemonset); err != nil {
			common.Log.Warnf("error updating %s: %v", daemonset.Name, err)
		}
	}

	deployments, err := h.deployments.List(secret.Namespace, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
		return nil, nil
	}
	for _, deployment := range deployments.Items {
		if _, err := h.processDeployment(&deployment); err != nil {
			common.Log.Warnf("error updating %s: %v", deployment.Name, err)
		}
	}

	statefulsets, err := h.statefulSets.List(secret.Namespace, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
		return nil, nil
	}
	for _, statefulset := range statefulsets.Items {
		if _, err := h.processStatefulSet(&statefulset); err != nil {
			common.Log.Warnf("error updating %s: %v", statefulset.Name, err)
		}
	}
	return nil, nil
}

func (h *Handler) processDaemonSet(daemonset *v1.DaemonSet) (*v1.DaemonSet, error) {
	if daemonset.Labels != nil && daemonset.Labels[dockhand.AutoUpdateLabelKey] == "true" {

		labels, annotations := h.getUpdatedLabelsAndAnnotations(
			daemonset.GetNamespace(),
			daemonset.GetLabels(),
			daemonset.Spec.Template.GetAnnotations())

		if val, ok := annotations[dockhand.SecretChecksumAnnotationKey]; ok && val != "" {
			var patch []k8s.PatchOperation
			patch = append(patch, k8s.GenerateSpecTemplateAnnotationPatch(daemonset.Spec.Template.GetAnnotations(), annotations)...)
			patch = append(patch, k8s.GenerateMetadataLabelsPatch(daemonset.GetLabels(), labels)...)
			patchBytes, _ := json.Marshal(patch)

			if _, err := h.daemonSets.Patch(daemonset.GetNamespace(), daemonset.GetName(), types.JSONPatchType, patchBytes); err != nil {
				common.Log.Warnf("unable to update %s error:[%v]", daemonset.GetName(), err)
			}
		}
	}
	return nil, nil
}

func (h *Handler) processDeployment(deployment *v1.Deployment) (*v1.Deployment, error) {
	if deployment.Labels != nil && deployment.Labels[dockhand.AutoUpdateLabelKey] == "true" {
		labels, annotations := h.getUpdatedLabelsAndAnnotations(
			deployment.GetNamespace(),
			deployment.GetLabels(),
			deployment.Spec.Template.GetAnnotations())

		if val, ok := annotations[dockhand.SecretChecksumAnnotationKey]; ok && val != "" {
			var patch []k8s.PatchOperation
			patch = append(patch, k8s.GenerateSpecTemplateAnnotationPatch(deployment.Spec.Template.GetAnnotations(), annotations)...)
			patch = append(patch, k8s.GenerateMetadataLabelsPatch(deployment.GetLabels(), labels)...)
			patchBytes, _ := json.Marshal(patch)

			if _, err := h.deployments.Patch(deployment.GetNamespace(), deployment.GetName(), types.JSONPatchType, patchBytes); err != nil {
				common.Log.Warnf("unable to update %s error:[%v]", deployment.GetName(), err)
			}
		}
	}
	return nil, nil
}

func (h *Handler) processStatefulSet(statefulset *v1.StatefulSet) (*v1.StatefulSet, error) {
	if statefulset.Labels != nil && statefulset.Labels[dockhand.AutoUpdateLabelKey] == "true" {
		labels, annotations := h.getUpdatedLabelsAndAnnotations(
			statefulset.GetNamespace(),
			statefulset.GetLabels(),
			statefulset.Spec.Template.GetAnnotations())

		if val, ok := annotations[dockhand.SecretChecksumAnnotationKey]; ok && val != "" {
			var patch []k8s.PatchOperation
			patch = append(patch, k8s.GenerateSpecTemplateAnnotationPatch(statefulset.Spec.Template.GetAnnotations(), annotations)...)
			patch = append(patch, k8s.GenerateMetadataLabelsPatch(statefulset.GetLabels(), labels)...)
			patchBytes, _ := json.Marshal(patch)

			if _, err := h.statefulSets.Patch(statefulset.GetNamespace(), statefulset.GetName(), types.JSONPatchType, patchBytes); err != nil {
				common.Log.Warnf("unable to update %s error:[%v]", statefulset.GetName(), err)
			}
		}
	}
	return nil, nil
}

func (h *Handler) onDaemonSetChange(_ string, daemonset *v1.DaemonSet) (*v1.DaemonSet, error) {
	if daemonset == nil {
		return nil, nil
	}
	return h.processDaemonSet(daemonset)
}

func (h *Handler) onDeploymentChange(_ string, deployment *v1.Deployment) (*v1.Deployment, error) {
	if deployment == nil {
		return nil, nil
	}
	return h.processDeployment(deployment)
}

func (h *Handler) onStatefulSetChange(_ string, statefulset *v1.StatefulSet) (*v1.StatefulSet, error) {
	if statefulset == nil {
		return nil, nil
	}
	return h.processStatefulSet(statefulset)
}

func (h *Handler) loadDockhandProfile(profile *dockhand.DockhandProfile) error {
	if profile.AwsSecretsManager != nil {
		aws.Region = profile.AwsSecretsManager.Region
		if profile.AwsSecretsManager.AccessKeyId != nil {
			aws.AccessKeyID = *profile.AwsSecretsManager.AccessKeyId
		}
		if profile.AwsSecretsManager.SecretAccessKeyRef != nil {
			secretData, _ := h.secrets.Get(h.operatorNamespace, profile.AwsSecretsManager.SecretAccessKeyRef.Name, metav1.GetOptions{})
			if secretData != nil {
				aws.SecretAccessKey = string(secretData.Data[profile.AwsSecretsManager.SecretAccessKeyRef.Key])
			}
		}
		if aws.AccessKeyID != "" && aws.SecretAccessKey != "" {
			aws.UseChainCredentials = false
		}
	}

	if profile.AzureKeyVault != nil {
		azure.KeyVaultName = profile.AzureKeyVault.KeyVault
		azure.TenantID = profile.AzureKeyVault.Tenant

		if profile.AzureKeyVault.ClientId != nil {
			azure.ClientID = *profile.AzureKeyVault.ClientId
		}

		if profile.AzureKeyVault.ClientSecretRef != nil {
			secretData, _ := h.secrets.Get(h.operatorNamespace, profile.AzureKeyVault.ClientSecretRef.Name, metav1.GetOptions{})
			if secretData != nil {
				azure.ClientSecret = string(secretData.Data[profile.AzureKeyVault.ClientSecretRef.Key])
			}
		}
		_ = os.Setenv("AZURE_TENANT_ID", azure.TenantID)
		_ = os.Setenv("AZURE_CLIENT_ID", azure.ClientID)
		_ = os.Setenv("AZURE_CLIENT_SECRET", azure.ClientSecret)
	}

	if profile.GcpSecretsManager != nil {
		gcp.Project = profile.GcpSecretsManager.Project
		if profile.GcpSecretsManager.CredentialsFileSecretRef != nil {
			secretData, _ := h.secrets.Get(h.operatorNamespace, profile.GcpSecretsManager.CredentialsFileSecretRef.Name, metav1.GetOptions{})

			if secretData != nil {
				gcp.CredentialsJson = secretData.Data[profile.GcpSecretsManager.CredentialsFileSecretRef.Key]
			}
		}
	}

	if profile.Vault != nil {
		vault.Addr = profile.Vault.Addr
		if profile.Vault.RoleId != nil {
			vault.RoleID = *profile.Vault.RoleId
		}
		if profile.Vault.SecretIdRef != nil {
			secretData, _ := h.secrets.Get(h.operatorNamespace, profile.Vault.SecretIdRef.Name, metav1.GetOptions{})
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

func (h *Handler) getUpdatedLabelsAndAnnotations(
	namespace string,
	labels map[string]string,
	annotations map[string]string) (map[string]string, map[string]string) {

	updatedLabels := k8s.CopyStringMap(labels)
	updatedAnnotations := k8s.CopyStringMap(annotations)

	var secrets []string
	if val, ok := updatedAnnotations[dockhand.SecretNamesAnnotationKey]; ok {
		secrets = strings.Split(val, ",")
	}
	var dhSecrets []string
	for _, name := range secrets {
		if secret, err := h.secrets.Get(namespace, name, metav1.GetOptions{}); err == nil {
			if val, ok := secret.Labels[dockhand.DockhandSecretLabelKey]; ok {
				dhSecrets = append(dhSecrets, val)
			}
		}
	}
	sort.Strings(dhSecrets)
	for key, label := range updatedLabels {
		if strings.HasPrefix(label, dockhand.DockhandSecretNamesLabelPrefixKey) {
			delete(updatedLabels, key)
		}
	}
	for _, dhSecret := range dhSecrets {
		updatedLabels[dockhand.DockhandSecretNamesLabelPrefixKey+dhSecret] = "true"
	}

	checksum, err := k8s.GetSecretsChecksum(h.ctx, secrets, namespace)
	if err != nil {
		common.Log.Warnf("unable to get checksum secrets=%s in namespace=%s with error[%v]", secrets, namespace, err)
	}
	updatedAnnotations[dockhand.SecretChecksumAnnotationKey] = checksum

	return updatedLabels, updatedAnnotations
}
