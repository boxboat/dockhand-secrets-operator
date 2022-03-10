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
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/boxboat/dockcmd/cmd/aws"
	"github.com/boxboat/dockcmd/cmd/azure"
	dockcmdCommon "github.com/boxboat/dockcmd/cmd/common"
	"github.com/boxboat/dockcmd/cmd/gcp"
	"github.com/boxboat/dockcmd/cmd/vault"
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dhs.dockhand.dev/v1alpha2"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	dockhandcontrollers "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dhs.dockhand.dev/v1alpha2"
	"github.com/boxboat/dockhand-secrets-operator/pkg/k8s"
	appscontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/kv"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
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
	secrets                    corecontrollers.SecretController
	recorder                   record.EventRecorder
	crossNamespaceAuthorized   bool
	awsProfileMap              map[string]*aws.SecretsClient
	azureProfileMap            map[string]*azure.SecretsClient
	gcpProfileMap              map[string]*gcp.SecretsClient
	vaultProfileMap            map[string]*vault.SecretsClient
}

const (
	recreateSeconds        = 30
	syncChangedSeconds     = 5
	minSyncIntervalSeconds = 5
)

func Register(
	ctx context.Context,
	namespace string,
	events typedcorev1.EventInterface,
	daemonsets appscontrollers.DaemonSetController,
	deployments appscontrollers.DeploymentController,
	statefulsets appscontrollers.StatefulSetController,
	secrets corecontrollers.SecretController,
	dockhandSecrets dockhandcontrollers.SecretController,
	dockhandProfile dockhandcontrollers.ProfileController,
	crossNamespaceAuthorized bool) {

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
		recorder:                   buildEventRecorder(events),
		crossNamespaceAuthorized:   crossNamespaceAuthorized,
		awsProfileMap:              make(map[string]*aws.SecretsClient),
		azureProfileMap:            make(map[string]*azure.SecretsClient),
		gcpProfileMap:              make(map[string]*gcp.SecretsClient),
		vaultProfileMap:            make(map[string]*vault.SecretsClient),
	}

	// Register handlers
	dockhandSecrets.OnChange(ctx, "dockhandsecret-onchange", h.onDockhandSecretChange)
	dockhandSecrets.OnRemove(ctx, "dockhandsecret-onremove", h.onDockhandSecretRemove)
	secrets.OnChange(ctx, "secrets-onchange", h.onManagedSecretChange)
	daemonsets.OnChange(ctx, "daemonsets-onchange", h.onDaemonSetChange)
	deployments.OnChange(ctx, "deployment-onchange", h.onDeploymentChange)
	statefulsets.OnChange(ctx, "statefulsets-onchange", h.onStatefulSetChange)
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

// onManagedSecretChange handler to re-sync Dockhand Secret to managed secret when it is externally deleted or modified.
func (h *Handler) onManagedSecretChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil {
		common.Log.Debugf("checking deleted secret %s", key)
		namespace, name := kv.Split(key, "/")
		dhsList, err := h.dhSecretsController.List(namespace, metav1.ListOptions{})
		common.LogIfError(err)
		for _, dhs := range dhsList.Items {
			if dhs.SecretSpec.Name == name && dhs.DeletionTimestamp == nil {
				common.Log.Infof("managed secret %s deleted - enqueuing dockhand secret %s/%s after %d seconds", key, dhs.Namespace, dhs.Name, recreateSeconds)
				h.dhSecretsController.EnqueueAfter(dhs.Namespace, dhs.Name, time.Second*recreateSeconds)
			}
		}
		return nil, nil
	}
	if secret.Labels != nil {
		common.Log.Debugf("%s secret change", key)
		if val, ok := secret.Labels[dockhand.DockhandSecretLabelKey]; ok {
			if dhSecret, err := h.dhSecretsController.Get(secret.Namespace, val, metav1.GetOptions{}); err == nil {
				common.Log.Infof("managed secret %s changed - enqueuing dockhand secret %s/%s after %d seconds", key, dhSecret.Namespace, dhSecret.Name, syncChangedSeconds)
				h.dhSecretsController.EnqueueAfter(dhSecret.Namespace, dhSecret.Name, time.Second*syncChangedSeconds)
			} else {
				common.LogIfError(err)
			}
		}
	}
	return nil, nil
}

// onDockhandSecretRemove delete managed Secret when Dockhand Secret is removed.
func (h *Handler) onDockhandSecretRemove(_ string, secret *dockhand.Secret) (*dockhand.Secret, error) {
	if secret == nil {
		return nil, nil
	}
	common.Log.Infof("dockhand secret removed %s/%s", secret.Namespace, secret.Name)
	common.Log.Infof("removing managed secret %s/%s", secret.Namespace, secret.SecretSpec.Name)
	if err := h.secrets.Delete(secret.Namespace, secret.SecretSpec.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		common.Log.Warnf(
			"could not delete secret=%s from namespace=%s",
			secret.SecretSpec.Name,
			secret.Namespace)
		return nil, err
	}

	return nil, nil
}

// onDockhandSecretChange handler responsible for creating/updating managed Secrets.
func (h *Handler) onDockhandSecretChange(_ string, secret *dockhand.Secret) (*dockhand.Secret, error) {
	// secret has been deleted so just return
	if secret == nil {
		return nil, nil
	}

	// secret is being deleted just return
	if secret.DeletionTimestamp != nil {
		return nil, nil
	}

	// Ready Secret, Generation and observedGeneration match - no change necessarily required
	if secret.Generation == secret.Status.ObservedGeneration && secret.Status.State == dockhand.Ready {
		updateRequired := false

		// check for syncInterval setting
		if syncDuration, err := time.ParseDuration(secret.SyncInterval); err == nil && syncDuration.Seconds() > 0 {
			if syncDuration.Seconds() < minSyncIntervalSeconds {
				syncDuration = minSyncIntervalSeconds * time.Second
				common.Log.Warnf("syncInterval for %s/%s < %ds, min %v will be used", secret.Namespace, secret.Name, minSyncIntervalSeconds, syncDuration)
				h.recorder.Eventf(secret, corev1.EventTypeWarning, "Warn", "syncInterval < %ds, min %v will be used", minSyncIntervalSeconds, syncDuration)
			}
			if lastSyncTime, err := time.Parse(time.RFC3339, secret.Status.SyncTimestamp); err == nil {
				nowTime := time.Now()
				nextSync := lastSyncTime.Add(syncDuration)
				// sync update is required
				if nextSync.Before(nowTime) {
					updateRequired = true
				} else {
					// re-enque after remaining time to next update
					syncDuration = nextSync.Sub(nowTime) + minSyncIntervalSeconds*time.Second
				}
			}

			common.Log.Debugf("enqueing %s/%s for sync after %s", secret.Namespace, secret.Name, syncDuration.String())
			h.dhSecretsController.EnqueueAfter(secret.Namespace, secret.Name, syncDuration)
		} else {
			common.Log.Debugf("%s metadata.generation[%d]==status.observedGeneration[%d]", secret.Name, secret.Generation, secret.Status.ObservedGeneration)
			if managedSecret, err := h.secrets.Get(secret.Namespace, secret.SecretSpec.Name, metav1.GetOptions{}); err == nil {
				if managedSecret.ResourceVersion != secret.Status.ObservedSecretResourceVersion {
					updateRequired = true
				}
			} else {
				updateRequired = true
			}
		}

		if !updateRequired {
			common.Log.Debugf("skipping update %s", secret.Name)
			return nil, nil
		}
	}

	common.Log.Debugf("Secret change: %v", secret)
	profileNamespace := secret.Namespace
	if secret.Profile.Namespace != "" {
		profileNamespace = secret.Profile.Namespace
	}
	if secret.Namespace != profileNamespace && !h.crossNamespaceAuthorized {
		err := fmt.Errorf(
			"could not access Profile[%s] in external namespace %s, cross namespace profile access is disabled",
			secret.Profile,
			profileNamespace)
		h.recorder.Eventf(
			secret,
			corev1.EventTypeWarning,
			"ErrUnauthorized",
			"Could not access Profile[%s] in external namespace %s",
			secret.Profile,
			profileNamespace)
		statusErr := h.updateDockhandSecretStatus(secret, nil, dockhand.ErrApplied)
		common.LogIfError(statusErr)
		return nil, err
	}
	profile, err := h.dhProfileCache.Get(profileNamespace, secret.Profile.Name)

	if err != nil {
		common.Log.Warnf("could not get profile %s/%s", profileNamespace, secret.Profile.Name)
		h.recorder.Eventf(
			secret,
			corev1.EventTypeWarning,
			"ErrLoadingProfile",
			"Could not get profile %s/%s",
			profileNamespace,
			secret.Profile)
		statusErr := h.updateDockhandSecretStatus(secret, nil, dockhand.ErrApplied)
		common.LogIfError(statusErr)
		return nil, err
	}

	profileFunctionMap, err := h.getProfileFuncMap(profile)
	if err != nil {
		h.recorder.Eventf(secret, corev1.EventTypeWarning, "ErrLoadingProfile", "Could not load Profile: %v", err)
		statusErr := h.updateDockhandSecretStatus(secret, nil, dockhand.ErrApplied)
		common.LogIfError(statusErr)
		return nil, err
	}

	k8sCacheSecret, err := h.secrets.Get(secret.Namespace, secret.SecretSpec.Name, metav1.GetOptions{})

	var k8sSecret *corev1.Secret

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
		k8sSecret = k8sCacheSecret.DeepCopy()
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

	// Store reference in K8s Secret to owning Dockhand Secret
	k8sSecret.Labels[dockhand.DockhandSecretLabelKey] = secret.Name

	// clear data
	k8sSecret.Data = make(map[string][]byte)
	for k, v := range secret.Data {

		secretData, err := dockcmdCommon.ParseSecretsTemplate([]byte(v), profileFunctionMap)

		if err != nil {
			h.recorder.Eventf(secret, corev1.EventTypeWarning, "ErrParsingSecret", "Could not parse template %v", err)
			statusErr := h.updateDockhandSecretStatus(secret, nil, dockhand.ErrApplied)
			common.LogIfError(statusErr)
			return nil, err
		}
		common.Log.Debugf("%s: %s", k, secretData)
		k8sSecret.Data[k] = secretData
	}

	var managedSecretUpdate *corev1.Secret

	if newSecret {
		if managedSecretUpdate, err = h.secrets.Create(k8sSecret); err == nil {
			h.recorder.Eventf(secret, corev1.EventTypeNormal, "Success", "Secret %s/%s created", secret.Namespace, secret.SecretSpec.Name)
		} else {
			h.recorder.Eventf(secret, corev1.EventTypeWarning, "Error", "Secret %s/%s not created", secret.Namespace, secret.SecretSpec.Name)
			statusErr := h.updateDockhandSecretStatus(secret, nil, dockhand.ErrApplied)
			common.LogIfError(statusErr)
			return nil, err
		}
	} else {
		currVersion := k8sSecret.ResourceVersion
		if managedSecretUpdate, err = h.secrets.Update(k8sSecret); err == nil {
			if managedSecretUpdate.ResourceVersion != currVersion {
				h.recorder.Eventf(secret, corev1.EventTypeNormal, "Success", "Secret %s/%s updated", secret.Namespace, secret.SecretSpec.Name)
			}
		} else {
			h.recorder.Eventf(secret, corev1.EventTypeWarning, "Error", "Secret %s/%s not updated", secret.Namespace, secret.SecretSpec.Name)
			statusErr := h.updateDockhandSecretStatus(secret, nil, dockhand.ErrApplied)
			common.LogIfError(statusErr)
			return nil, err
		}
	}

	// if we have made it here the secret is provisioned and ready
	if err := h.updateDockhandSecretStatus(secret, managedSecretUpdate, dockhand.Ready); err != nil {
		// log status update error but continue
		common.LogIfError(err)
	}

	h.updateDeployments(secret.Name, secret.Namespace)
	h.updateDaemonSets(secret.Name, secret.Namespace)
	h.updateStatefulSets(secret.Name, secret.Namespace)

	return nil, nil
}

// processDaemonSet handler checks DaemonSets for the AutoUpdateLabel and if it is set to true will determine if any
// of the referenced secrets have been modified.
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
				return nil, err
			}
		}
	}
	return nil, nil
}

// processDeployment checks Deployments for the AutoUpdateLabel and if it is set to true will determine if any
// of the referenced secrets have been modified.
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
				return nil, err
			}
		}
	}
	return nil, nil
}

// processStatefulSet checks StatefulSets for the AutoUpdateLabel and if it is set to true will determine if any
// of the referenced secrets have been modified.
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
				return nil, err
			}
		}
	}
	return nil, nil
}

// updateStatefulSets updates statefulsets in the provided namespace if they reference a dockhand secret
func (h *Handler) updateStatefulSets(dockhandSecretName string, namespace string) {
	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + dockhandSecretName

	if statefulsets, err := h.statefulSets.List(namespace, metav1.ListOptions{LabelSelector: labelSelector}); err == nil {
		for _, statefulset := range statefulsets.Items {
			if _, err := h.processStatefulSet(&statefulset); err != nil {
				common.Log.Warnf("error updating %s: %v", statefulset.Name, err)
			}
		}
	} else {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
	}
}

// updateDeployments updates deployments in the provided namespace if they reference a dockhand secret
func (h *Handler) updateDeployments(dockhandSecretName, namespace string) {
	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + dockhandSecretName

	if deployments, err := h.deployments.List(namespace, metav1.ListOptions{LabelSelector: labelSelector}); err == nil {
		for _, deployment := range deployments.Items {
			if _, err := h.processDeployment(&deployment); err != nil {
				common.Log.Warnf("error updating %s: %v", deployment.Name, err)
			}
		}
	} else {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
	}
}

// updateDaemonSets updates daemonsets in the provided namespace if they reference a dockhand secret
func (h *Handler) updateDaemonSets(dockhandSecretName, namespace string) {
	labelSelector := dockhand.DockhandSecretNamesLabelPrefixKey + dockhandSecretName

	if daemonsets, err := h.daemonSets.List(namespace, metav1.ListOptions{LabelSelector: labelSelector}); err == nil {
		for _, daemonset := range daemonsets.Items {
			if _, err := h.processDaemonSet(&daemonset); err != nil {
				common.Log.Warnf("error updating %s: %v", daemonset.Name, err)
			}
		}
	} else {
		common.Log.Warnf("error listing deployments associated with %s: %v", labelSelector, err)
	}
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

func (h *Handler) getProfileFuncMap(profile *dockhand.Profile) (template.FuncMap, error) {
	profileName := profile.Namespace + "/" + profile.Name

	funcMap := make(template.FuncMap)

	if profile.AwsSecretsManager != nil {
		client, ok := h.awsProfileMap[profileName]
		if !ok {
			common.Log.Debugf("creating new aws client for %s", profileName)
			opts := []aws.SecretsClientOpt{aws.Region(profile.AwsSecretsManager.Region)}

			if cacheTTL, err := time.ParseDuration(profile.AwsSecretsManager.CacheTTL); err == nil {
				opts = append(opts, aws.CacheTTL(cacheTTL))
			} else {
				return nil, err
			}
			accessKeyID := ""
			secretAccessKey := ""
			if profile.AwsSecretsManager.AccessKeyId != nil {
				accessKeyID = *profile.AwsSecretsManager.AccessKeyId
			}

			if profile.AwsSecretsManager.SecretAccessKeyRef != nil {
				if secretData, err := h.secrets.Get(profile.Namespace, profile.AwsSecretsManager.SecretAccessKeyRef.Name, metav1.GetOptions{}); err == nil {
					if secretData != nil {
						secretAccessKey = string(secretData.Data[profile.AwsSecretsManager.SecretAccessKeyRef.Key])
					}
				} else {
					return nil, err
				}
			}
			if accessKeyID != "" && secretAccessKey != "" {
				opts = append(opts, aws.AccessKeyIDAndSecretAccessKey(accessKeyID, secretAccessKey))
			} else {
				opts = append(opts, aws.UseChainCredentials())
			}

			var err error
			client, err = aws.NewSecretsClient(opts...)
			if err != nil {
				return nil, err
			}
			h.awsProfileMap[profileName] = client
		}
		funcMap["aws"] = client.GetJSONSecret
		funcMap["awsJson"] = client.GetJSONSecret
		funcMap["awsText"] = client.GetTextSecret
	}

	if profile.AzureKeyVault != nil {
		client, ok := h.azureProfileMap[profileName]
		if !ok {
			common.Log.Debugf("creating new azure key vault client for %s", profileName)
			opts := []azure.SecretsClientOpt{
				azure.KeyVaultName(profile.AzureKeyVault.KeyVault),
				azure.TenantID(profile.AzureKeyVault.Tenant)}

			if cacheTTL, err := time.ParseDuration(profile.AzureKeyVault.CacheTTL); err == nil {
				opts = append(opts, azure.CacheTTL(cacheTTL))
			} else {
				return nil, err
			}

			clientID := ""
			clientSecret := ""
			if profile.AzureKeyVault.ClientId != nil {
				clientID = *profile.AzureKeyVault.ClientId
			}

			if profile.AzureKeyVault.ClientSecretRef != nil {
				if secretData, err := h.secrets.Get(profile.Namespace, profile.AzureKeyVault.ClientSecretRef.Name, metav1.GetOptions{}); err == nil {
					if secretData != nil {
						clientSecret = string(secretData.Data[profile.AzureKeyVault.ClientSecretRef.Key])
					}
				} else {
					return nil, err
				}
			}
			if clientID != "" && clientSecret != "" {
				opts = append(opts, azure.ClientIDAndSecret(clientID, clientSecret))
			} else {
				return nil, fmt.Errorf("no valid azure credentials provided in profile %s", profileName)
			}

			var err error
			client, err = azure.NewSecretsClient(opts...)
			if err != nil {
				return nil, err
			}
			h.azureProfileMap[profileName] = client

		}
		funcMap["azureJson"] = client.GetJSONSecret
		funcMap["azureText"] = client.GetTextSecret

	}

	if profile.GcpSecretsManager != nil {
		client, ok := h.gcpProfileMap[profileName]
		if !ok {
			common.Log.Debugf("creating new gcp client for %s", profileName)
			opts := []gcp.SecretsClientOpt{gcp.Project(profile.GcpSecretsManager.Project)}
			if cacheTTL, err := time.ParseDuration(profile.GcpSecretsManager.CacheTTL); err == nil {
				opts = append(opts, gcp.CacheTTL(cacheTTL))
			} else {
				return nil, err
			}
			if profile.GcpSecretsManager.CredentialsFileSecretRef != nil {
				if secretData, err := h.secrets.Get(profile.Namespace, profile.GcpSecretsManager.CredentialsFileSecretRef.Name, metav1.GetOptions{}); err == nil {
					if secretData != nil {
						opts = append(opts, gcp.CredentialsJson(secretData.Data[profile.GcpSecretsManager.CredentialsFileSecretRef.Key]))
					}
				} else {
					return nil, err
				}
			}
			var err error
			client, err = gcp.NewSecretsClient(h.ctx, opts...)
			if err != nil {
				return nil, err
			}
			h.gcpProfileMap[profileName] = client
		}
		funcMap["gcpJson"] = client.GetJSONSecret
		funcMap["gcpText"] = client.GetTextSecret
	}

	if profile.Vault != nil {

		client, ok := h.vaultProfileMap[profileName]
		if !ok {
			common.Log.Debugf("creating new vault client for %s", profileName)
			opts := []vault.SecretsClientOpt{vault.Address(profile.Vault.Addr)}
			if cacheTTL, err := time.ParseDuration(profile.Vault.CacheTTL); err == nil {
				opts = append(opts, vault.CacheTTL(cacheTTL))
			} else {
				return nil, err
			}

			roleID := ""
			secretID := ""
			if profile.Vault.RoleId != nil {
				roleID = *profile.Vault.RoleId
			}
			if profile.Vault.SecretIdRef != nil {
				if secretData, err := h.secrets.Get(profile.Namespace, profile.Vault.SecretIdRef.Name, metav1.GetOptions{}); err == nil {
					if secretData != nil {
						secretID = string(secretData.Data[profile.Vault.SecretIdRef.Key])
					}
				} else {
					return nil, err
				}
			}
			if roleID != "" && secretID != "" {
				opts = append(opts, vault.RoleAndSecretID(roleID, secretID))
			}
			if profile.Vault.TokenRef != nil {
				if secretData, err := h.secrets.Get(profile.Namespace, profile.Vault.TokenRef.Name, metav1.GetOptions{}); err == nil {
					if secretData != nil {
						opts = append(opts, vault.Token(string(secretData.Data[profile.Vault.TokenRef.Key])))
					}
				} else {
					return nil, err
				}
			}
			var err error
			client, err = vault.NewSecretsClient(opts...)
			if err != nil {
				return nil, err
			}
			h.vaultProfileMap[profileName] = client
		}
		funcMap["vault"] = client.GetJSONSecret
	}

	return funcMap, nil
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

func (h *Handler) updateDockhandSecretStatus(secret *dockhand.Secret, managedSecret *corev1.Secret, state dockhand.SecretState) error {
	common.Log.Debugf("updating %s status", secret.Name)
	secretCopy := secret.DeepCopy()
	secretCopy.Status.State = state
	// generation successfully processed so store observedGeneration
	if state == dockhand.Ready {
		secretCopy.Status.ObservedGeneration = secret.Generation
		secretCopy.Status.SyncTimestamp = time.Now().Format(time.RFC3339)
	}

	if managedSecret != nil {
		secretCopy.Status.ObservedSecretResourceVersion = managedSecret.ResourceVersion
	}

	_, err := h.dhSecretsController.UpdateStatus(secretCopy)

	return err
}
