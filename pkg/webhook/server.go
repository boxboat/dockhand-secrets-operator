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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
	"github.com/boxboat/dockhand-secrets-operator/pkg/common"
	"github.com/boxboat/dockhand-secrets-operator/pkg/k8s"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Server struct {
	Server        *http.Server
	runtimeScheme *runtime.Scheme
	codecs        serializer.CodecFactory
	deserializer  runtime.Decoder
}

type ServerParameters struct {
	port     int
	x509Cert string
	key      string
}

func (server *Server) Init() {
	server.runtimeScheme = runtime.NewScheme()
	server.codecs = serializer.NewCodecFactory(server.runtimeScheme)
	server.deserializer = server.codecs.UniversalDeserializer()
}

// Check labels for whether the target resource needs to be mutated
func mutationRequired(labels map[string]string) bool {
	inject := false
	if _, ok := labels[dockhand.AutoUpdateLabelKey]; ok {
		inject = true
	}
	return inject
}

// main mutation process
func (server *Server) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request

	if req.Kind.Kind == "DaemonSet" {
		return server.mutateDaemonSet(ar)
	} else if req.Kind.Kind == "Deployment" {
		return server.mutateDeployment(ar)
	} else if req.Kind.Kind == "StatefulSet" {
		return server.mutateStatefulSet(ar)
	}
	common.Log.Debugf("Unhandled kind presented for mutation for %v", req)
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func (server *Server) mutateDaemonSet(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	ds := &appsv1.DaemonSet{}

	if err := json.Unmarshal(req.Object.Raw, &ds); err != nil {
		common.Log.Errorf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	common.Log.Debugf(
		"AdmissionReview for Kind=%s, Namespace=%s Name=%s (%s) UID=%s patchOperation=%s UserInfo=%s",
		req.Kind,
		req.Namespace,
		req.Name,
		ds.ObjectMeta.Name,
		req.UID,
		req.Operation,
		req.UserInfo)

	common.Log.Debugf("ds.Labels[%v]", ds.Labels)

	// determine whether to perform mutation
	if !mutationRequired(ds.Labels) {
		common.Log.Debugf("Skipping mutation for %s/%s due to policy check", ds.Namespace, ds.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	labels, annotations := processDockhandSecretAnnotations(
		ds.Labels,
		ds.Spec.Template.Annotations,
		ds.Namespace,
		ds.Spec.Template.Spec)

	patchBytes, err := createDaemonSetPatch(ds, labels, annotations)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	common.Log.Debugf("AdmissionResponse: patch=[%s]", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		UID:     ar.Request.UID,
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (server *Server) mutateDeployment(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	deployment := &appsv1.Deployment{}

	if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
		common.Log.Errorf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	common.Log.Debugf(
		"AdmissionReview for Kind=%s, Namespace=%s Name=%s (%s) UID=%s patchOperation=%s UserInfo=%s",
		req.Kind,
		req.Namespace,
		req.Name,
		deployment.ObjectMeta.Name,
		req.UID,
		req.Operation,
		req.UserInfo)

	common.Log.Debugf("deployment.Labels[%v]", deployment.Labels)

	// determine whether to perform mutation
	if !mutationRequired(deployment.Labels) {
		common.Log.Debugf("Skipping mutation for %s/%s due to policy check", deployment.Namespace, deployment.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	labels, annotations := processDockhandSecretAnnotations(
		deployment.Labels,
		deployment.Spec.Template.Annotations,
		deployment.Namespace,
		deployment.Spec.Template.Spec)

	patchBytes, err := createDeploymentPatch(deployment, labels, annotations)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	common.Log.Debugf("AdmissionResponse: patch=[%s]", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		UID:     ar.Request.UID,
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (server *Server) mutateStatefulSet(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	statefulset := &appsv1.StatefulSet{}

	if err := json.Unmarshal(req.Object.Raw, &statefulset); err != nil {
		common.Log.Errorf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	common.Log.Debugf(
		"AdmissionReview for Kind=%s, Namespace=%s Name=%s (%s) UID=%s patchOperation=%s UserInfo=%s",
		req.Kind,
		req.Namespace,
		req.Name,
		statefulset.ObjectMeta.Name,
		req.UID,
		req.Operation,
		req.UserInfo)

	common.Log.Debugf("statefulset.Labels[%v]", statefulset.Labels)

	// determine whether to perform mutation
	if !mutationRequired(statefulset.Labels) {
		common.Log.Debugf("Skipping mutation for %s/%s due to policy check", statefulset.Namespace, statefulset.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}
	labels, annotations := processDockhandSecretAnnotations(
		statefulset.Labels,
		statefulset.Spec.Template.Annotations,
		statefulset.Namespace,
		statefulset.Spec.Template.Spec)

	patchBytes, err := createStatefulSetPatch(statefulset, labels, annotations)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	common.Log.Debugf("AdmissionResponse: patch=[%s]", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		UID:     ar.Request.UID,
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// Serve method for webhook server
func (server *Server) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		common.Log.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		common.Log.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := &admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, ar); err != nil {
		common.Log.Errorf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = server.mutate(ar)
	}

	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		common.Log.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	common.Log.Debugf("Ready to write response ...")
	if _, err := w.Write(resp); err != nil {
		common.Log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// create mutation patch for resources
func createDaemonSetPatch(daemonSet *appsv1.DaemonSet, labels map[string]string, annotations map[string]string) ([]byte, error) {
	var patch []k8s.PatchOperation
	patch = append(patch, k8s.GenerateSpecTemplateAnnotationPatch(daemonSet.Spec.Template.Annotations, annotations)...)
	patch = append(patch, k8s.GenerateMetadataLabelsPatch(daemonSet.Labels, labels)...)
	return json.Marshal(patch)
}

// create mutation patch for resources
func createDeploymentPatch(deployment *appsv1.Deployment, labels map[string]string, annotations map[string]string) ([]byte, error) {
	var patch []k8s.PatchOperation
	patch = append(patch, k8s.GenerateSpecTemplateAnnotationPatch(deployment.Spec.Template.Annotations, annotations)...)
	patch = append(patch, k8s.GenerateMetadataLabelsPatch(deployment.Labels, labels)...)
	return json.Marshal(patch)
}

func createStatefulSetPatch(statefulSet *appsv1.StatefulSet, labels map[string]string, annotations map[string]string) ([]byte, error) {
	var patch []k8s.PatchOperation
	patch = append(patch, k8s.GenerateSpecTemplateAnnotationPatch(statefulSet.Spec.Template.Annotations, annotations)...)
	patch = append(patch, k8s.GenerateMetadataLabelsPatch(statefulSet.Labels, labels)...)
	return json.Marshal(patch)
}

// Similar behavior to pkg.controller.getUpdatedLabelsAndAnnotations
func processDockhandSecretAnnotations(
	labels map[string]string,
	annotations map[string]string,
	namespace string,
	podSpec corev1.PodSpec) (map[string]string, map[string]string) {

	updatedLabels := k8s.CopyStringMap(labels)
	updatedAnnotations := k8s.CopyStringMap(annotations)

	secrets := getSecretsSetFromPodSpec(podSpec)
	sort.Strings(secrets)

	// block for no more than 15 seconds
	attempt := 0
	for {
		if checksum, err := k8s.GetSecretsChecksum(context.Background(), secrets, namespace); err == nil {
			updatedAnnotations[dockhand.SecretChecksumAnnotationKey] = checksum
			break
		} else {
			if attempt < 5 {
				common.Log.Warnf("unable to calculate checksum - retrying:[%v]", err)
			} else {
				common.Log.Warnf("unable to calculate checksum after 5th attempt:[%v]", err)
				updatedAnnotations[dockhand.SecretChecksumAnnotationKey] = ""
				break
			}
		}
		attempt += 1
		time.Sleep(3 * time.Second)
	}

	dhSecrets, err := k8s.GetDockhandSecretsListFromK8sSecrets(context.Background(), secrets, namespace)
	if err != nil {
		common.Log.Warnf("%v", err)
	}

	for key, label := range updatedLabels {
		if strings.HasPrefix(label, dockhand.DockhandSecretNamesLabelPrefixKey) {
			delete(updatedLabels, key)
		}
	}
	for _, dhSecret := range dhSecrets {
		updatedLabels[dockhand.DockhandSecretNamesLabelPrefixKey+dhSecret] = "true"
	}

	updatedAnnotations[dockhand.SecretNamesAnnotationKey] = strings.Join(secrets, ",")

	return updatedLabels, updatedAnnotations
}

func getSecretsSetFromPodSpec(podSpec corev1.PodSpec) []string {
	secretSet := make(map[string]struct{})

	var member struct{}

	for _, container := range podSpec.Containers {
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				secretSet[env.ValueFrom.SecretKeyRef.Name] = member
			}
		}
		for _, envSource := range container.EnvFrom {
			if envSource.SecretRef != nil {
				secretSet[envSource.SecretRef.Name] = member
			}
		}
	}

	for _, volume := range podSpec.Volumes {
		if volume.Secret != nil {
			secretSet[volume.Secret.SecretName] = member
		}
	}

	var secrets []string
	for key := range secretSet {
		secrets = append(secrets, key)
	}

	return secrets
}
