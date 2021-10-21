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

package v2

import (
	"context"
	"encoding/json"
	"github.com/boxboat/dockcmd/cmd/aws"
	"github.com/boxboat/dockcmd/cmd/azure"
	dockcmdCommon "github.com/boxboat/dockcmd/cmd/common"
	"github.com/boxboat/dockcmd/cmd/gcp"
	"github.com/boxboat/dockcmd/cmd/vault"
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dhs.dockhand.dev/v1alpha1"
	dockhandv1 "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	dockhandcontrollers "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dhs.dockhand.dev/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/k8s"
	appscontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"
)

// Handler is the controller implementation for Secret Resources
type Handler struct {
	ctx                        context.Context
	operatorNamespace          string
	daemonSets                 appscontrollers.DaemonSetClient
	deployments                appscontrollers.DeploymentClient
	dhSecretsController        dockhandcontrollers.SecretController
	dhSecretsProfileController dockhandcontrollers.ProfileController
	dhProfileCache             dockhandcontrollers.ProfileCache
	statefulSets               appscontrollers.StatefulSetClient
	secrets                    corecontrollers.SecretClient
	funcMap                    template.FuncMap
	recorder                   record.EventRecorder
}

func Register(
	ctx context.Context,
	namespace string,
	events typedcorev1.EventInterface,
	daemonsets appscontrollers.DaemonSetController,
	deployments appscontrollers.DeploymentController,
	statefulsets appscontrollers.StatefulSetController,
	secrets corecontrollers.SecretClient,
	dockhandSecrets dockhandcontrollers.SecretController,
	dockhandProfile dockhandcontrollers.ProfileController,
	funcMap template.FuncMap) *Handler {

	h := &Handler{
		ctx:                        ctx,
		operatorNamespace:          namespace,
		daemonSets:                 daemonsets,
		deployments:                deployments,
		dhSecretsController:        dockhandSecrets,
		dhSecretsProfileController: dockhandProfile,
		dhProfileCache:             dockhandProfile.Cache(),
		secrets:                    secrets,
		statefulSets:               statefulsets,
		funcMap:                    funcMap,
		recorder:                   buildEventRecorder(events),
	}

	// Register handlers
	dockhandSecrets.OnChange(ctx, "dockhandsecret-onchange", h.onDockhandSecretChange)
	dockhandSecrets.OnRemove(ctx, "dockhandsecret-onremove", h.onDockhandSecretRemove)
	daemonsets.OnChange(ctx, "daemonsets-onchange", h.onDaemonSetChange)
	deployments.OnChange(ctx, "deployment-onchange", h.onDeploymentChange)
	statefulsets.OnChange(ctx, "statefulsets-onchange", h.onStatefulSetChange)

	return h
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

func (h *Handler) onDockhandSecretRemove(_ string, secret *dockhand.Secret) (*dockhand.Secret, error) {
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

func (h *Handler) onDockhandSecretChange(_ string, secret *dockhand.Secret) (*dockhand.Secret, error) {
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
	k8sSecret.Labels[dockhand.DockhandSecretLabelKey] = secret.SecretSpec.Name

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
		if _, err := h.secrets.Update(k8sSecret); err == nil{
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

	h.UpdateDeployments(secret.SecretSpec.Name, secret.Namespace)
	h.UpdateDaemonSets(secret.SecretSpec.Name, secret.Namespace)
	h.UpdateStatefulSets(secret.SecretSpec.Name, secret.Namespace)

	return nil, nil
}

func (h *Handler) ProcessDaemonSet(daemonset *v1.DaemonSet) (*v1.DaemonSet, error) {
	// TODO remove v1
	if daemonset.Labels != nil && daemonset.Labels[dockhand.AutoUpdateLabelKey] == "true" || daemonset.Labels[dockhandv1.AutoUpdateLabelKey] == "true"{

		labels, annotations := h.GetUpdatedLabelsAndAnnotations(
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
				return nil, err
			}
		}
	}
	return nil, nil
}

func (h *Handler) ProcessDeployment(deployment *v1.Deployment) (*v1.Deployment, error) {
	// TODO remove v1
	if deployment.Labels != nil && deployment.Labels[dockhand.AutoUpdateLabelKey] == "true" || deployment.Labels[dockhandv1.AutoUpdateLabelKey] == "true"{
		labels, annotations := h.GetUpdatedLabelsAndAnnotations(
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
				return nil, err
			}
		}
	}
	return nil, nil
}

// UpdateStatefulSets updates statefulsets in the provided namespace if they reference a dockhand secret
func (h *Handler) UpdateStatefulSets(dockhandSecretName string, namespace string) {
	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + dockhandSecretName

	if statefulsets, err := h.statefulSets.List(namespace, metav1.ListOptions{LabelSelector: labelSelector}); err == nil {
		for _, statefulset := range statefulsets.Items {
			if _, err := h.ProcessStatefulSet(&statefulset); err != nil {
				common.Log.Warnf("error updating %s: %v", statefulset.Name, err)
			}
		}
	} else {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
	}
}

// UpdateDeployments updates deployments in the provided namespace if they reference a dockhand secret
func (h *Handler) UpdateDeployments(dockhandSecretName, namespace string) {
	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + dockhandSecretName

	if deployments, err := h.deployments.List(namespace, metav1.ListOptions{LabelSelector: labelSelector}); err == nil {
		for _, deployment := range deployments.Items {
			if _, err := h.ProcessDeployment(&deployment); err != nil {
				common.Log.Warnf("error updating %s: %v", deployment.Name, err)
			}
		}
	} else {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
	}
}

// UpdateDaemonSets updates daemonsets in the provided namespace if they reference a dockhand secret
func (h *Handler) UpdateDaemonSets(dockhandSecretName, namespace string) {
	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + dockhandSecretName

	if daemonsets, err := h.daemonSets.List(namespace, metav1.ListOptions{LabelSelector: labelSelector}); err == nil {
		for _, daemonset := range daemonsets.Items {
			if _, err := h.ProcessDaemonSet(&daemonset); err != nil {
				common.Log.Warnf("error updating %s: %v", daemonset.Name, err)
			}
		}
	} else {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
	}
}

func (h *Handler) ProcessStatefulSet(statefulset *v1.StatefulSet) (*v1.StatefulSet, error) {
	// TODO remove v1
	if statefulset.Labels != nil && statefulset.Labels[dockhand.AutoUpdateLabelKey] == "true" || statefulset.Labels[dockhandv1.AutoUpdateLabelKey] == "true"{
		labels, annotations := h.GetUpdatedLabelsAndAnnotations(
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
				return nil, err
			}
		}
	}
	return nil, nil
}

func (h *Handler) onDaemonSetChange(_ string, daemonset *v1.DaemonSet) (*v1.DaemonSet, error) {
	if daemonset == nil {
		return nil, nil
	}
	return h.ProcessDaemonSet(daemonset)
}

func (h *Handler) onDeploymentChange(_ string, deployment *v1.Deployment) (*v1.Deployment, error) {
	if deployment == nil {
		return nil, nil
	}
	return h.ProcessDeployment(deployment)
}

func (h *Handler) onStatefulSetChange(_ string, statefulset *v1.StatefulSet) (*v1.StatefulSet, error) {
	if statefulset == nil {
		return nil, nil
	}
	return h.ProcessStatefulSet(statefulset)
}

func (h *Handler) loadDockhandSecretsProfile(profile *dockhand.Profile) error {
	if profile.AwsSecretsManager != nil {
		var err error
		if aws.CacheTTL, err = time.ParseDuration(profile.AwsSecretsManager.CacheTTL); err != nil{
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
		if azure.CacheTTL, err = time.ParseDuration(profile.AzureKeyVault.CacheTTL); err != nil{
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
		if gcp.CacheTTL, err = time.ParseDuration(profile.GcpSecretsManager.CacheTTL); err != nil{
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
		if vault.CacheTTL, err = time.ParseDuration(profile.Vault.CacheTTL); err != nil{
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

func (h *Handler) GetUpdatedLabelsAndAnnotations(
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

func (h*Handler) updateDockhandSecretStatus(secret *dockhand.Secret, state dockhand.SecretState) error {
	common.Log.Infof("Updating %s status", secret.Name)
	secretCopy := secret.DeepCopy()
	secretCopy.Status.State = state
	_, err := h.dhSecretsController.UpdateStatus(secretCopy)

	return err
}
