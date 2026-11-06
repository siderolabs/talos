// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"fmt"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// reservedSELinuxDomains names the admission policy which keeps the SELinux domains of Talos to the components it runs
// in them, and the binding which enforces it.
const reservedSELinuxDomains = "talos-reserved-selinux-domains"

// podSELinuxTypes lists in CEL the SELinux types a pod selects: the type of the pod and the type of each of its
// containers, empty when unset.
const podSELinuxTypes = `[object.spec.?securityContext.?seLinuxOptions.?type.orValue("")] +
object.spec.containers.map(c, c.?securityContext.?seLinuxOptions.?type.orValue("")) +
object.spec.?initContainers.orValue([]).map(c, c.?securityContext.?seLinuxOptions.?type.orValue("")) +
object.spec.?ephemeralContainers.orValue([]).map(c, c.?securityContext.?seLinuxOptions.?type.orValue(""))`

// selinuxType returns the type of an SELinux label.
func selinuxType(label string) string {
	return strings.Split(label, ":")[2]
}

// ReservedSELinuxDomainsPolicyTemplate returns the admission policy which refuses to pods and ephemeral containers the
// SELinux domains of the control plane, and those of the system services the CRI runs for Talos: etcd, the kubelet,
// the Talos containers and the CRI itself. The mirror pods the kubelets create for the static pods keep their domains:
// the kubelet never runs a mirror pod, it runs the static pod from its file.
func ReservedSELinuxDomainsPolicyTemplate() runtime.Object {
	domains := []string{
		constants.KubernetesAPIServerSELinuxType,
		constants.KubernetesControllerManagerSELinuxType,
		constants.KubernetesSchedulerSELinuxType,
		selinuxType(constants.SELinuxLabelEtcd),
		selinuxType(constants.SelinuxLabelKubelet),
		selinuxType(constants.SelinuxLabelTalosContainer),
		selinuxType(constants.SelinuxLabelTalosContainerMachined),
		selinuxType(constants.SelinuxLabelPodRuntime),
	}

	// the security context of a pod is immutable: the API server checks a pod when it is created, and again when it
	// gets an ephemeral container
	rule := func(operation admissionregistrationv1.OperationType, resource string) admissionregistrationv1.NamedRuleWithOperations {
		return admissionregistrationv1.NamedRuleWithOperations{
			Operations:  []admissionregistrationv1.OperationType{operation},
			APIGroups:   []string{corev1.GroupName},
			APIVersions: []string{corev1.SchemeGroupVersion.Version},
			Resources:   []string{resource},
		}
	}

	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		Kind:       "ValidatingAdmissionPolicy",
		APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
		Name:       reservedSELinuxDomains,
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: new(admissionregistrationv1.Fail),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					rule(admissionregistrationv1.Create, "pods"),
					rule(admissionregistrationv1.Update, "pods/ephemeralcontainers"),
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name: "not-a-mirror-pod",
					Expression: fmt.Sprintf(`!(request.userInfo.username.startsWith("system:node:") && `+
						`has(object.metadata.annotations) && %q in object.metadata.annotations)`, corev1.MirrorPodAnnotationKey),
				},
			},
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "selinuxTypes",
					Expression: podSELinuxTypes,
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: fmt.Sprintf(`!variables.selinuxTypes.exists(t, t in ["%s"])`, strings.Join(domains, `", "`)),
					Message:    fmt.Sprintf("the SELinux types %s are reserved for the system services and the static pods of Talos", strings.Join(domains, ", ")),
					Reason:     new(metav1.StatusReasonForbidden),
				},
			},
		},
	}
}

// ReservedSELinuxDomainsPolicyBindingTemplate returns the binding which enforces the admission policy on the SELinux
// domains of Talos across the cluster.
func ReservedSELinuxDomainsPolicyBindingTemplate() runtime.Object {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		Kind:       "ValidatingAdmissionPolicyBinding",
		APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
		Name:       reservedSELinuxDomains,
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        reservedSELinuxDomains,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
}
