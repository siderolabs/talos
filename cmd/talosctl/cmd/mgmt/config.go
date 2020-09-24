// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/talos-systems/talos/pkg/images"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/bundle"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	additionalSANs    []string
	configVersion     string
	architecture      string
	dnsDomain         string
	kubernetesVersion string
	installDisk       string
	installImage      string
	outputDir         string
	registryMirrors   []string
	persistConfig     bool
)

// genConfigCmd represents the gen config command.
var genConfigCmd = &cobra.Command{
	Use:   "config <cluster name> <cluster endpoint>",
	Short: "Generates a set of configuration files for Talos cluster",
	Long: `The cluster endpoint is the URL for the Kubernetes API. If you decide to use
	a control plane node, common in a single node control plane setup, use port 6443 as
	this is the port that the API server binds to on every control plane node. For an HA
	setup, usually involving a load balancer, use the IP and port of the load balancer.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate url input to ensure it has https:// scheme before we attempt to gen
		u, err := url.Parse(args[1])
		if err != nil {
			return fmt.Errorf("failed to parse load balancer IP or DNS name: %w", err)
		}
		if u.Scheme == "" {
			return fmt.Errorf("no scheme specified for load balancer IP or DNS name\ntry \"https://<load balancer IP or DNS name>\"")
		}

		switch configVersion {
		case "v1alpha1":
			return genV1Alpha1Config(args)
		}

		return nil
	},
}

//nolint: gocyclo
func genV1Alpha1Config(args []string) error {
	// If output dir isn't specified, set to the current working dir
	var err error
	if outputDir == "" {
		outputDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working dir: %w", err)
		}
	}

	// Create dir path, ignoring "already exists" messages
	if err = os.MkdirAll(outputDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	var genOptions []generate.GenOption //nolint: prealloc

	for _, registryMirror := range registryMirrors {
		components := strings.SplitN(registryMirror, "=", 2)
		if len(components) != 2 {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		genOptions = append(genOptions, generate.WithRegistryMirror(components[0], components[1]))
	}

	configBundle, err := bundle.NewConfigBundle(
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: args[0],
				Endpoint:    args[1],
				KubeVersion: kubernetesVersion,
				GenOptions: append(genOptions,
					generate.WithInstallDisk(installDisk),
					generate.WithInstallImage(installImage),
					generate.WithAdditionalSubjectAltNames(additionalSANs),
					generate.WithDNSDomain(dnsDomain),
					generate.WithPersist(persistConfig),
					generate.WithArchitecture(architecture),
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("failed to generate config bundle: %w", err)
	}

	for _, t := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeJoin} {
		name := strings.ToLower(t.String()) + ".yaml"
		fullFilePath := filepath.Join(outputDir, name)

		var configString string

		switch t { //nolint: exhaustive
		case machine.TypeInit:
			configString, err = configBundle.Init().String()
			if err != nil {
				return err
			}
		case machine.TypeControlPlane:
			configString, err = configBundle.ControlPlane().String()
			if err != nil {
				return err
			}
		case machine.TypeJoin:
			configString, err = configBundle.Join().String()
			if err != nil {
				return err
			}
		}

		if err = ioutil.WriteFile(fullFilePath, []byte(configString), 0o644); err != nil {
			return err
		}

		fmt.Printf("created %s\n", fullFilePath)
	}

	// We set the default endpoint to localhost for configs generated, with expectation user will tweak later
	configBundle.TalosConfig().Contexts[args[0]].Endpoints = []string{"127.0.0.1"}

	data, err := yaml.Marshal(configBundle.TalosConfig())
	if err != nil {
		return fmt.Errorf("failed to marshal config: %+v", err)
	}

	fullFilePath := filepath.Join(outputDir, "talosconfig")

	if err = ioutil.WriteFile(fullFilePath, data, 0o644); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("created %s\n", fullFilePath)

	return nil
}

func init() {
	genCmd.AddCommand(genConfigCmd)
	genConfigCmd.Flags().StringVar(&installDisk, "install-disk", "/dev/sda", "the disk to install to")
	genConfigCmd.Flags().StringVar(&installImage, "install-image", helpers.DefaultImage(images.DefaultInstallerImageRepository), "the image used to perform an installation")
	genConfigCmd.Flags().StringSliceVar(&additionalSANs, "additional-sans", []string{}, "additional Subject-Alt-Names for the APIServer certificate")
	genConfigCmd.Flags().StringVar(&dnsDomain, "dns-domain", "cluster.local", "the dns domain to use for cluster")
	genConfigCmd.Flags().StringVar(&architecture, "arch", runtime.GOARCH, "the architecture of the cluster")
	genConfigCmd.Flags().StringVar(&configVersion, "version", "v1alpha1", "the desired machine config version to generate")
	genConfigCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	genConfigCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "destination to output generated files")
	genConfigCmd.Flags().StringSliceVar(&registryMirrors, "registry-mirror", []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	genConfigCmd.Flags().BoolVarP(&persistConfig, "persist", "p", true, "the desired persist value for configs")
}
