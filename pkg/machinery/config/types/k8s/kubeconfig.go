// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"go.yaml.in/yaml/v4"
)

// webhookKubeconfig is the subset of the kubeconfig format which is validated for webhook kubeconfigs.
type webhookKubeconfig struct {
	CurrentContext string                     `yaml:"current-context"`
	Clusters       []webhookKubeconfigCluster `yaml:"clusters"`
	Contexts       []webhookKubeconfigContext `yaml:"contexts"`
	Users          []webhookKubeconfigUser    `yaml:"users"`
}

type webhookKubeconfigCluster struct {
	Name    string         `yaml:"name"`
	Cluster map[string]any `yaml:"cluster"`
}

type webhookKubeconfigContext struct {
	Name    string `yaml:"name"`
	Context struct {
		Cluster string `yaml:"cluster"`
	} `yaml:"context"`
}

type webhookKubeconfigUser struct {
	Name string         `yaml:"name"`
	User map[string]any `yaml:"user"`
}

// webhookKubeconfigDeniedClusterFields are cluster fields which reference files on the host, mapped to the inline alternative.
//
// kube-apiserver can't read them, as only the kubeconfig itself is rendered into its static pod.
var webhookKubeconfigDeniedClusterFields = map[string]string{
	"certificate-authority": "certificate-authority-data",
}

// webhookKubeconfigDeniedUserFields are user fields which reference files or binaries on the host, mapped to the inline alternative.
var webhookKubeconfigDeniedUserFields = map[string]string{
	"client-certificate": "client-certificate-data",
	"client-key":         "client-key-data",
	"tokenFile":          "token",
	"exec":               "",
	"auth-provider":      "",
}

// validateWebhookKubeconfig validates the kubeconfig kube-apiserver uses to connect to a webhook.
func validateWebhookKubeconfig(kubeconfig string) error {
	var cfg webhookKubeconfig

	if err := yaml.Unmarshal([]byte(kubeconfig), &cfg); err != nil {
		return fmt.Errorf("kubeconfig is not valid: %w", err)
	}

	var errs error

	for _, cluster := range cfg.Clusters {
		errs = errors.Join(errs, validateDeniedKubeconfigFields("cluster", cluster.Name, cluster.Cluster, webhookKubeconfigDeniedClusterFields))
	}

	for _, user := range cfg.Users {
		errs = errors.Join(errs, validateDeniedKubeconfigFields("user", user.Name, user.User, webhookKubeconfigDeniedUserFields))
	}

	if cfg.CurrentContext == "" {
		return errors.Join(errs, errors.New("kubeconfig current-context is required"))
	}

	contextIdx := slices.IndexFunc(cfg.Contexts, func(c webhookKubeconfigContext) bool { return c.Name == cfg.CurrentContext })
	if contextIdx == -1 {
		return errors.Join(errs, fmt.Errorf("kubeconfig current-context %q is not defined in contexts", cfg.CurrentContext))
	}

	clusterName := cfg.Contexts[contextIdx].Context.Cluster

	clusterIdx := slices.IndexFunc(cfg.Clusters, func(c webhookKubeconfigCluster) bool { return c.Name == clusterName })
	if clusterIdx == -1 {
		return errors.Join(errs, fmt.Errorf("kubeconfig cluster %q referenced by the current context is not defined in clusters", clusterName))
	}

	if server, _ := cfg.Clusters[clusterIdx].Cluster["server"].(string); server == "" { //nolint:errcheck
		errs = errors.Join(errs, fmt.Errorf("kubeconfig cluster %q has no server", clusterName))
	}

	return errs
}

func validateDeniedKubeconfigFields(kind, name string, fields map[string]any, denied map[string]string) error {
	var errs error

	for _, field := range slices.Sorted(maps.Keys(denied)) {
		if _, ok := fields[field]; !ok {
			continue
		}

		if alternative := denied[field]; alternative != "" {
			errs = errors.Join(errs, fmt.Errorf("kubeconfig %s %q: %q is not supported, as kube-apiserver can't access host files, use %q instead", kind, name, field, alternative))
		} else {
			errs = errors.Join(errs, fmt.Errorf("kubeconfig %s %q: %q is not supported, as kube-apiserver can't access host files", kind, name, field))
		}
	}

	return errs
}
