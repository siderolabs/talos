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

// controlPlaneSELinuxDomains names the admission policy which keeps the SELinux domains of the control plane to its
// static pods, and the binding which enforces it.
const controlPlaneSELinuxDomains = "talos-control-plane-selinux-domains"

// podSELinuxTypes lists in CEL the SELinux types a pod selects: the type of the pod and the type of each of its
// containers, empty when unset.
const podSELinuxTypes = `[object.spec.?securityContext.?seLinuxOptions.?type.orValue("")] +
object.spec.containers.map(c, c.?securityContext.?seLinuxOptions.?type.orValue("")) +
object.spec.?initContainers.orValue([]).map(c, c.?securityContext.?seLinuxOptions.?type.orValue("")) +
object.spec.?ephemeralContainers.orValue([]).map(c, c.?securityContext.?seLinuxOptions.?type.orValue(""))`

// ControlPlaneSELinuxDomainsPolicyTemplate returns the admission policy which refuses the SELinux domains of the control
// plane to pods and ephemeral containers. The mirror pods the kubelets create for the static pods keep them: the kubelet
// never runs a mirror pod, it runs the static pod from its file.
func ControlPlaneSELinuxDomainsPolicyTemplate() runtime.Object {
	domains := []string{
		constants.KubernetesAPIServerSELinuxType,
		constants.KubernetesControllerManagerSELinuxType,
		constants.KubernetesSchedulerSELinuxType,
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
		Name:       controlPlaneSELinuxDomains,
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
					Message:    fmt.Sprintf("the SELinux types %s are reserved for the static pods of the control plane", strings.Join(domains, ", ")),
					Reason:     new(metav1.StatusReasonForbidden),
				},
			},
		},
	}
}

// ControlPlaneSELinuxDomainsPolicyBindingTemplate returns the binding which enforces the admission policy on SELinux
// domains of the control plane across the cluster.
func ControlPlaneSELinuxDomainsPolicyBindingTemplate() runtime.Object {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		Kind:       "ValidatingAdmissionPolicyBinding",
		APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
		Name:       controlPlaneSELinuxDomains,
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        controlPlaneSELinuxDomains,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
}
