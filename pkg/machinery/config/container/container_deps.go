// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// validateContainerDependencies checks that `dependsOn.containers` references resolve and form a DAG.
//
// Catching this when the configuration is applied matters: two containers waiting on each other
// would otherwise boot the node into a state where both sit in `pending` forever, which is
// considerably harder to diagnose on a live machine than a rejected `talosctl apply-config`.
func validateContainerDependencies(configs []config.ContainerConfig) error {
	if len(configs) == 0 {
		return nil
	}

	names, edges, errs := buildContainerDependencyGraph(configs)

	if cycles := detectContainerDependencyCycles(names, edges); len(cycles) > 0 {
		errs = multierror.Append(errs,
			fmt.Errorf("container dependsOn cycle detected: %s", strings.Join(cycles, ", ")))
	}

	if errs == nil {
		return nil
	}

	if multiErr, ok := errors.AsType[*multierror.Error](errs); ok {
		return multiErr.ErrorOrNil()
	}

	return errs
}

// buildContainerDependencyGraph builds the dependsOn adjacency list, reporting any dependency that
// references a container which isn't configured.
func buildContainerDependencyGraph(configs []config.ContainerConfig) (map[string]struct{}, map[string][]string, error) {
	names := make(map[string]struct{}, len(configs))
	for _, cfg := range configs {
		names[cfg.Name()] = struct{}{}
	}

	var errs error

	edges := make(map[string][]string, len(configs))

	for _, cfg := range configs {
		deps := cfg.DependsOn().Containers()

		for _, dep := range deps {
			if _, exists := names[dep]; !exists {
				errs = multierror.Append(errs,
					fmt.Errorf("container %q depends on container %q, which is not configured", cfg.Name(), dep))
			}
		}

		edges[cfg.Name()] = deps
	}

	return names, edges, errs
}

// detectContainerDependencyCycles walks the dependency graph and returns each cycle found as a
// sorted, deduped "a -> b -> a" chain.
func detectContainerDependencyCycles(names map[string]struct{}, edges map[string][]string) []string {
	// Iterate over sorted names so a given configuration always reports the same cycle.
	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}

	slices.Sort(sortedNames)

	walker := &containerDependencyCycleWalker{
		edges:     edges,
		names:     names,
		visiting:  make(map[string]bool, len(names)),
		permanent: make(map[string]bool, len(names)),
	}

	for _, name := range sortedNames {
		walker.visit(name)
	}

	slices.Sort(walker.cycles)

	return slices.Compact(walker.cycles)
}

// containerDependencyCycleWalker holds the DFS state for detectContainerDependencyCycles.
type containerDependencyCycleWalker struct {
	edges map[string][]string
	names map[string]struct{}
	// visiting tracks the current DFS stack, permanent tracks fully explored nodes.
	visiting  map[string]bool
	permanent map[string]bool
	stack     []string
	cycles    []string
}

func (w *containerDependencyCycleWalker) visit(name string) {
	if w.permanent[name] {
		return
	}

	if w.visiting[name] {
		// Report the cycle starting from where it closes, so the message reads as a loop.
		if idx := slices.Index(w.stack, name); idx >= 0 {
			w.cycles = append(w.cycles, strings.Join(append(slices.Clone(w.stack[idx:]), name), " -> "))
		}

		return
	}

	w.visiting[name] = true
	w.stack = append(w.stack, name)

	for _, dep := range w.edges[name] {
		if _, exists := w.names[dep]; !exists {
			// Already reported as unresolved by buildContainerDependencyGraph; don't walk into it.
			continue
		}

		w.visit(dep)
	}

	w.stack = w.stack[:len(w.stack)-1]
	w.visiting[name] = false
	w.permanent[name] = true
}
