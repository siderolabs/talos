// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inject

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	injectToEnv = false
	volumeName  = "talos-secrets"

	nameSuffix = "-talos-secrets"

	apiVersionField = "apiVersion"
	kindField       = "kind"
	metadataField   = "metadata"
	namespaceField  = "namespace"
	nameField       = "name"

	yamlSeparator = "---\n"
)

// ServiceAccount takes a YAML with Kubernetes manifests and requested Talos roles as input
// and injects Talos service accounts into them.
//
//nolint:gocyclo
func ServiceAccount(reader io.Reader, roles []string) ([]byte, error) {
	var err error

	objectSerializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		nil,
		nil,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)

	seenResourceIDs := make(map[string]struct{})

	var buf bytes.Buffer

	decoder := yaml.NewDecoder(reader)

	// loop over all documents in a possibly YAML with multiple documents separated by ---
	for {
		var raw map[string]any

		err = decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}

		if raw == nil {
			continue
		}

		var injected metav1.Object

		injected, err = injectToObject(raw)
		if err != nil { // not a known resource with a PodSpec
			// if this is already a Talos ServiceAccount resource we have seen,
			// we keep it only if we have not seen it yet (means it belongs to the user, not injected by us)
			id := readResourceIDFromServiceAccount(raw)
			if id != "" {
				if _, ok := seenResourceIDs[id]; ok {
					continue
				}

				seenResourceIDs[id] = struct{}{}
			}

			err = yaml.NewEncoder(&buf).Encode(raw)
			if err != nil {
				return nil, err
			}

			buf.WriteString(yamlSeparator)

			continue
		}

		// injectable resource type which contains a PodSpec

		runtimeObject, ok := injected.(runtime.Object)
		if !ok {
			return nil, errors.New("injected object is not a runtime.Object")
		}

		err = objectSerializer.Encode(runtimeObject, &buf)
		if err != nil {
			return nil, err
		}

		buf.WriteString(yamlSeparator)

		id := readResourceIDFromObject(injected)

		// inject service account for the resource
		if _, ok = seenResourceIDs[id]; !ok {
			sa := buildServiceAccount(injected.GetNamespace(), fmt.Sprintf("%s%s", injected.GetName(), nameSuffix), roles)

			err = yaml.NewEncoder(&buf).Encode(sa)
			if err != nil {
				return nil, err
			}

			buf.WriteString(yamlSeparator)

			// mark resource as seen
			seenResourceIDs[id] = struct{}{}
		}
	}

	return buf.Bytes(), nil
}

func buildServiceAccount(namespace string, name string, roles []string) map[string]any {
	metadata := map[string]any{
		nameField: name,
	}

	if namespace != "" {
		metadata[namespaceField] = namespace
	}

	return map[string]any{
		apiVersionField: fmt.Sprintf(
			"%s/%s",
			constants.ServiceAccountResourceGroup,
			constants.ServiceAccountResourceVersion,
		),
		kindField:     constants.ServiceAccountResourceKind,
		metadataField: metadata,
		"spec": map[string]any{
			"roles": roles,
		},
	}
}

func isServiceAccount(raw map[string]any) bool {
	apiVersionKind, err := readResourceAPIVersionKind(raw)
	if err != nil {
		return false
	}

	return apiVersionKind == fmt.Sprintf(
		"%s/%s/%s",
		constants.ServiceAccountResourceGroup,
		constants.ServiceAccountResourceVersion,
		constants.ServiceAccountResourceKind,
	)
}

// injectToDocument takes a single YAML document and attempts to inject a ServiceAccount
// into it if it is a known Kubernetes resource type which contains a corev1.PodSpec.
func injectToObject(raw map[string]any) (metav1.Object, error) {
	var err error

	apiVersionKind, err := readResourceAPIVersionKind(raw)
	if err != nil {
		return nil, err
	}

	switch apiVersionKind {
	case "v1/Pod":
		return injectToPodSpecObject[corev1.Pod](raw, func(obj *corev1.Pod) *corev1.PodSpec {
			return &obj.Spec
		})

	case "apps/v1/Deployment":
		return injectToPodSpecObject[appsv1.Deployment](raw, func(obj *appsv1.Deployment) *corev1.PodSpec {
			return &obj.Spec.Template.Spec
		})

	case "apps/v1/StatefulSet":
		return injectToPodSpecObject[appsv1.StatefulSet](raw, func(obj *appsv1.StatefulSet) *corev1.PodSpec {
			return &obj.Spec.Template.Spec
		})

	case "apps/v1/DaemonSet":
		return injectToPodSpecObject[appsv1.DaemonSet](raw, func(obj *appsv1.DaemonSet) *corev1.PodSpec {
			return &obj.Spec.Template.Spec
		})

	case "batch/v1/Job":
		return injectToPodSpecObject[batchv1.Job](raw, func(obj *batchv1.Job) *corev1.PodSpec {
			return &obj.Spec.Template.Spec
		})

	case "batch/v1/CronJob":
		return injectToPodSpecObject[batchv1.CronJob](raw, func(obj *batchv1.CronJob) *corev1.PodSpec {
			return &obj.Spec.JobTemplate.Spec.Template.Spec
		})
	}

	return nil, fmt.Errorf("unsupported object type: %s", apiVersionKind)
}

func injectToPodSpecObject[T any](raw map[string]any, podSpecFunc func(*T) *corev1.PodSpec) (*T, error) {
	objectName, nameFound, err := unstructured.NestedString(raw, metadataField, nameField)
	if err != nil {
		return nil, err
	}

	if !nameFound {
		return nil, errors.New("object has no name")
	}

	var obj T

	err = runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(raw, &obj, false)
	if err != nil {
		return nil, err
	}

	injectToPodSpec(fmt.Sprintf("%s%s", objectName, nameSuffix), podSpecFunc(&obj))

	return &obj, nil
}

func readResourceAPIVersionKind(raw map[string]any) (string, error) {
	apiVersion, found, err := unstructured.NestedString(raw, apiVersionField)
	if err != nil {
		return "", err
	}

	if !found {
		return "", fmt.Errorf("%s not found", apiVersionField)
	}

	kind, found, err := unstructured.NestedString(raw, kindField)
	if err != nil {
		return "", err
	}

	if !found {
		return "", fmt.Errorf("%s not found", kindField)
	}

	return fmt.Sprintf("%s/%s", apiVersion, kind), nil
}

func readResourceIDFromObject(obj metav1.Object) string {
	if obj.GetNamespace() == "" {
		return obj.GetName()
	}

	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}

func readResourceIDFromServiceAccount(raw map[string]any) string {
	if !isServiceAccount(raw) {
		return ""
	}

	name, nameFound, err := unstructured.NestedString(raw, metadataField, nameField)
	if err != nil || !nameFound {
		return ""
	}

	nameTrimmed := strings.TrimSuffix(name, nameSuffix)

	ns, nsFound, err := unstructured.NestedString(raw, metadataField, namespaceField)
	if err != nil {
		return ""
	}

	if nsFound {
		return fmt.Sprintf("%s/%s", ns, nameTrimmed)
	}

	return nameTrimmed
}

func injectToPodSpec(secretName string, podSpec *corev1.PodSpec) {
	podSpec.Volumes = injectToVolumes(secretName, podSpec.Volumes)
	podSpec.InitContainers = injectToContainers(podSpec.InitContainers)
	podSpec.Containers = injectToContainers(podSpec.Containers)
}

func injectToVolumes(name string, volumes []corev1.Volume) []corev1.Volume {
	result := make([]corev1.Volume, 0, len(volumes))

	for _, volume := range volumes {
		if volume.Name != volumeName {
			result = append(result, volume)
		}
	}

	result = append(result, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	})

	return result
}

func injectToContainers(containers []corev1.Container) []corev1.Container {
	result := make([]corev1.Container, 0, len(containers))

	for _, container := range containers {
		injectToContainer(&container)

		result = append(result, container)
	}

	return result
}

func injectToContainer(container *corev1.Container) {
	volumeMounts := make([]corev1.VolumeMount, 0, len(container.VolumeMounts))

	for _, mount := range container.VolumeMounts {
		if mount.Name != volumeName {
			volumeMounts = append(volumeMounts, mount)
		}
	}

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: constants.ServiceAccountMountPath,
	})

	container.VolumeMounts = volumeMounts

	if injectToEnv {
		container.Env = injectToContainerEnv(container.Env)
	}
}

func injectToContainerEnv(env []corev1.EnvVar) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(env))

	for _, envVar := range env {
		if envVar.Name != constants.TalosConfigEnvVar {
			result = append(result, envVar)
		}
	}

	result = append(result, corev1.EnvVar{
		Name:  constants.TalosConfigEnvVar,
		Value: filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
	})

	return result
}
