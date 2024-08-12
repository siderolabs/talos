// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build generate

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/siderolabs/gen/xslices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const sourceURL = "https://raw.githubusercontent.com/flannel-io/flannel/%s/Documentation/kube-flannel.yml"

const configMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-flannel-cfg
  namespace: kube-system
  labels:
    tier: node
    k8s-app: flannel
data:
  cni-conf.json: |
    {
      "name": "cbr0",
      "cniVersion": "1.0.0",
      "plugins": [
        {
          "type": "flannel",
          "delegate": {
            "hairpinMode": true,
            "isDefaultGateway": true
          }
        },
        {
          "type": "portmap",
          "capabilities": {
            "portMappings": true
          }
        }
      ]
    }
  net-conf.json: |
    {
      {{- $hasIPv4 := false }}
      {{- range $cidr := .PodCIDRs }}
        {{- if contains $cidr "." }}
        {{- $hasIPv4 = true }}
      "Network": "{{ $cidr }}",
        {{- else }}
      "IPv6Network": "{{ $cidr }}",
      "EnableIPv6": true,
        {{- end }}
      {{- end }}
      {{- if not $hasIPv4 }}
      "EnableIPv4": false,
      {{- end }}
      "Backend": {
        "Type": "vxlan",
        "Port": 4789
      }
    }
---
`

func marshal(out io.Writer, obj runtime.Object) {
	m, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}

	m = regexp.MustCompile(` +creationTimestamp: null\n`).ReplaceAll(m, nil)
	m = regexp.MustCompile(`status:\n(  .+\n)+`).ReplaceAll(m, nil)

	m = regexp.MustCompile(`( +)- EXTRA_ARGS_PLACEHOLDER`).ReplaceAll(m, []byte("$1{{- range $$arg := .FlannelExtraArgs }}\n$1- {{ $$arg | json }}\n$1{{- end }}"))
	m = regexp.MustCompile(`( +)- name: EXTRA_ENV_PLACEHOLDER`).ReplaceAll(m, []byte("$1{{- if .FlannelKubeServiceHost }}\n$1- name: KUBERNETES_SERVICE_HOST\n$1  value: {{ .FlannelKubeServiceHost | json }}\n$1{{- end }}\n$1{{- if .FlannelKubeServicePort }}\n$1- name: KUBERNETES_SERVICE_PORT\n$1  value: {{ .FlannelKubeServicePort | json }}\n$1{{- end }}"))

	fmt.Fprintf(out, "%s---\n", string(m))
}

func main() {
	url := fmt.Sprintf(sourceURL, constants.FlannelVersion)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	manifest, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("template.go")
	if err != nil {
		log.Fatal(err)
	}

	defer out.Close()

	fmt.Fprintf(out, `// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Code generated from the manifest %s DO NOT EDIT

package flannel

// Template is a flannel manifest template.
var Template = []byte(`+"`", url)

	decoder := scheme.Codecs.UniversalDeserializer()

	for _, resourceYAML := range strings.Split(string(manifest), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err := decoder.Decode(
			[]byte(resourceYAML),
			nil,
			nil)
		if err != nil {
			log.Fatal(err)
		}

		switch groupVersionKind.Kind {
		case "Namespace":
			continue
		case "ClusterRole":
			marshal(out, obj)
		case "ClusterRoleBinding":
			crb := obj.(*rbacv1.ClusterRoleBinding)
			crb.Subjects[0].Namespace = "kube-system"
			crb.CreationTimestamp = metav1.Time{}

			marshal(out, obj)
		case "ServiceAccount":
			sa := obj.(*corev1.ServiceAccount)
			sa.Namespace = "kube-system"

			marshal(out, obj)
		case "ConfigMap":
			fmt.Fprint(out, configMap)
		case "DaemonSet":
			ds := obj.(*appsv1.DaemonSet)
			ds.Namespace = "kube-system"
			ds.Name = "kube-flannel"
			ds.Status = appsv1.DaemonSetStatus{}
			ds.Labels["k8s-app"] = "flannel"
			delete(ds.Labels, "app")

			ds.Spec.Template.Labels["k8s-app"] = "flannel"
			delete(ds.Spec.Template.Labels, "app")

			ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations,
				corev1.Toleration{
					Effect:   "NoExecute",
					Operator: "Exists",
				})

			ds.Spec.Selector.MatchLabels["k8s-app"] = "flannel"
			ds.Spec.Selector.MatchLabels["tier"] = "node"
			delete(ds.Spec.Selector.MatchLabels, "app")

			ds.Spec.Template.Spec.Containers[0].Image = "{{ .FlannelImage }}"
			ds.Spec.Template.Spec.Containers[0].Args = append(ds.Spec.Template.Spec.Containers[0].Args,
				"EXTRA_ARGS_PLACEHOLDER")

			ds.Spec.Template.Spec.Containers[0].Env = append(ds.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "EXTRA_ENV_PLACEHOLDER"})

			ds.Spec.Template.Spec.Volumes = xslices.FilterInPlace(ds.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool {
				return v.Name != "xtables-lock"
			})
			ds.Spec.Template.Spec.Containers[0].VolumeMounts = xslices.FilterInPlace(
				ds.Spec.Template.Spec.Containers[0].VolumeMounts, func(v corev1.VolumeMount) bool {
					return v.Name != "xtables-lock"
				})

			ds.Spec.Template.Spec.InitContainers = []corev1.Container{
				{
					Name:    "install-config",
					Image:   "{{ .FlannelImage }}",
					Command: []string{"cp"},
					Args: []string{
						"-f",
						"/etc/kube-flannel/cni-conf.json",
						"/etc/cni/net.d/10-flannel.conflist",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "cni",
							MountPath: "/etc/cni/net.d",
						},
						{
							Name:      "flannel-cfg",
							MountPath: "/etc/kube-flannel/",
						},
					},
				},
			}

			marshal(out, obj)
		default:
			log.Fatalf("unknown resource kind: %q", groupVersionKind.Kind)
		}

	}

	fmt.Fprint(out, "`)\n")
}
