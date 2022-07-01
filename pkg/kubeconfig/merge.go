// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Merger handles merging of Kubernetes client config files.
type Merger clientcmdapi.Config

// Load the kubeconfig from file.
func Load(path string) (*Merger, error) {
	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	return (*Merger)(config), err
}

// MergeOptions controls Merge process.
type MergeOptions struct {
	ForceContextName string
	ActivateContext  bool
	ConflictHandler  func(ConfigComponent, string) (ConflictDecision, error)
	OutputWriter     io.Writer
}

// ConfigComponent identifies part of kubeconfig.
type ConfigComponent string

// Kubeconfig components.
const (
	Cluster  ConfigComponent = "cluster"
	AuthInfo ConfigComponent = "auth"
	Context  ConfigComponent = "context"
)

// ConflictDecision is returned from ConflictHandler.
type ConflictDecision string

// Conflict decisions.
const (
	OverwriteDecision ConflictDecision = "overwrite"
	RenameDecision    ConflictDecision = "rename"
)

// Merge the provided kubernetes config in.
//
//nolint:gocyclo,cyclop
func (merger *Merger) Merge(config *clientcmdapi.Config, options MergeOptions) error {
	mappedClusters := map[string]string{}
	mappedAuthInfos := map[string]string{}
	mappedContexts := map[string]string{}

	for name, newCluster := range config.Clusters {
		mergedName := name

		oldCluster, exists := merger.Clusters[mergedName]

		newCluster.LocationOfOrigin = ""

		if oldCluster != nil {
			oldCluster.LocationOfOrigin = ""
		}

		if exists && !reflect.DeepEqual(oldCluster, newCluster) {
			decision, err := options.ConflictHandler(Cluster, name)
			if err != nil {
				return err
			}

			if decision == RenameDecision {
				mergedName = merger.rename(Cluster, mergedName)
			}
		}

		mappedClusters[name] = mergedName
	}

	for name, newAuthInfo := range config.AuthInfos {
		mergedName := name

		// apply previous mappings done to cluster names
		for oldName, newName := range mappedClusters {
			mergedName = strings.ReplaceAll(mergedName, oldName, newName)
		}

		oldAuthInfo, exists := merger.AuthInfos[mergedName]

		newAuthInfo.LocationOfOrigin = ""

		if oldAuthInfo != nil {
			oldAuthInfo.LocationOfOrigin = ""
		}

		if exists && !reflect.DeepEqual(oldAuthInfo, newAuthInfo) {
			decision, err := options.ConflictHandler(AuthInfo, name)
			if err != nil {
				return err
			}

			if decision == RenameDecision {
				mergedName = merger.rename(AuthInfo, mergedName)
			}
		}

		mappedAuthInfos[name] = mergedName
	}

	for name, newContext := range config.Contexts {
		mergedName := name

		// apply mappings done to authInfo, as authInfo has same format as context in Talos
		for oldName, newName := range mappedAuthInfos {
			mergedName = strings.ReplaceAll(mergedName, oldName, newName)
		}

		if options.ForceContextName != "" {
			mergedName = options.ForceContextName
		}

		oldContext, exists := merger.Clusters[mergedName]

		newContext.LocationOfOrigin = ""

		if oldContext != nil {
			oldContext.LocationOfOrigin = ""
		}

		if exists && !reflect.DeepEqual(oldContext, newContext) {
			decision, err := options.ConflictHandler(Cluster, name)
			if err != nil {
				return err
			}

			if decision == RenameDecision {
				mergedName = merger.rename(Cluster, mergedName)
			}
		}

		mappedContexts[name] = mergedName
	}

	for name, cluster := range config.Clusters {
		newName := mappedClusters[name]

		if newName != name {
			fmt.Fprintf(options.OutputWriter, "renamed cluster %q -> %q\n", name, newName)
		}

		merger.Clusters[newName] = cluster
	}

	for name, authInfo := range config.AuthInfos {
		newName := mappedAuthInfos[name]

		if newName != name {
			fmt.Fprintf(options.OutputWriter, "renamed auth info %q -> %q\n", name, newName)
		}

		merger.AuthInfos[newName] = authInfo
	}

	for name, context := range config.Contexts {
		contextCopy := *context

		newName := mappedContexts[name]

		if newName != name {
			fmt.Fprintf(options.OutputWriter, "renamed context %q -> %q\n", name, newName)
		}

		contextCopy.AuthInfo = mappedAuthInfos[contextCopy.AuthInfo]
		contextCopy.Cluster = mappedClusters[contextCopy.Cluster]

		merger.Contexts[newName] = &contextCopy

		if options.ActivateContext {
			merger.CurrentContext = newName
		}
	}

	return nil
}

// rename the config component until it gets unique.
func (merger *Merger) rename(component ConfigComponent, name string) (newName string) {
	i := 0
	newName = name

	for {
		var exists bool

		switch component {
		case Cluster:
			_, exists = merger.Clusters[newName]
		case AuthInfo:
			_, exists = merger.AuthInfos[newName]
		case Context:
			_, exists = merger.Contexts[newName]
		}

		if !exists {
			return newName
		}

		i++
		newName = fmt.Sprintf("%s-%d", name, i)
	}
}

// Write the kubeconfig back to the file.
func (merger *Merger) Write(path string) error {
	return clientcmd.WriteToFile(clientcmdapi.Config(*merger), path)
}
