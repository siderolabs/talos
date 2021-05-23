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
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/spf13/cobra"
	talosnet "github.com/talos-systems/net"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/talos-systems/talos/pkg/images"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/encoder"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/bundle"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var genConfigCmdFlags struct {
	additionalSANs          []string
	configVersion           string
	dnsDomain               string
	kubernetesVersion       string
	talosVersion            string
	installDisk             string
	installImage            string
	outputDir               string
	configPatch             string
	configPatchControlPlane string
	configPatchJoin         string
	registryMirrors         []string
	persistConfig           bool
	withExamples            bool
	withDocs                bool
}

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
			if !strings.Contains(args[1], "/") {
				// not a URL, could be just host:port
				u = &url.URL{
					Host: args[1],
				}
			} else {
				return fmt.Errorf("failed to parse the cluster endpoint URL: %w", err)
			}
		}

		if u.Scheme == "" {
			if u.Port() == "" {
				return fmt.Errorf("no scheme and port specified for the cluster endpoint URL\ntry: %q", fixControlPlaneEndpoint(u))
			}

			return fmt.Errorf("no scheme specified for the cluster endpoint URL\ntry: %q", fixControlPlaneEndpoint(u))
		}

		if u.Scheme != "https" {
			return fmt.Errorf("the control plane endpoint URL should have scheme https://\ntry: %q", fixControlPlaneEndpoint(u))
		}

		if err = talosnet.ValidateEndpointURI(args[1]); err != nil {
			return fmt.Errorf("error validating the cluster endpoint URL: %w", err)
		}

		switch genConfigCmdFlags.configVersion {
		case "v1alpha1":
			return writeV1Alpha1Config(args)
		}

		return nil
	},
}

func fixControlPlaneEndpoint(u *url.URL) *url.URL {
	// handle the case when the hostname/IP is given without the port, it parses as URL Path
	if u.Scheme == "" && u.Host == "" && u.Path != "" {
		u.Host = u.Path
		u.Path = ""
	}

	u.Scheme = "https"

	if u.Port() == "" {
		u.Host = fmt.Sprintf("%s:%d", u.Host, constants.DefaultControlPlanePort)
	}

	return u
}

// GenV1Alpha1Config generates the Talos config bundle
//
// GenV1Alpha1Config is useful for integration with external tooling options.
func GenV1Alpha1Config(genOptions []generate.GenOption,
	clusterName string,
	endpoint string,
	kubernetesVersion string,
	configPatch string,
	configPatchControlPlane string,
	configPatchJoin string) (*v1alpha1.ConfigBundle, error) {
	configBundleOpts := []bundle.Option{
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    endpoint,
				KubeVersion: strings.TrimPrefix(kubernetesVersion, "v"),
				GenOptions:  genOptions,
			},
		),
	}

	addConfigPatch := func(configPatch string, configOpt func(jsonpatch.Patch) bundle.Option) error {
		if configPatch == "" {
			return nil
		}

		jsonPatch, err := jsonpatch.DecodePatch([]byte(configPatch))
		if err != nil {
			return fmt.Errorf("error parsing config JSON patch: %w", err)
		}

		configBundleOpts = append(configBundleOpts, configOpt(jsonPatch))

		return nil
	}

	if err := addConfigPatch(configPatch, bundle.WithJSONPatch); err != nil {
		return nil, err
	}

	if err := addConfigPatch(configPatchControlPlane, bundle.WithJSONPatchControlPlane); err != nil {
		return nil, err
	}

	if err := addConfigPatch(configPatchJoin, bundle.WithJSONPatchJoin); err != nil {
		return nil, err
	}

	configBundle, err := bundle.NewConfigBundle(configBundleOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate config bundle: %w", err)
	}

	// We set the default endpoint to localhost for configs generated, with expectation user will tweak later
	configBundle.TalosConfig().Contexts[clusterName].Endpoints = []string{"127.0.0.1"}

	return configBundle, nil
}

//nolint:gocyclo
func writeV1Alpha1Config(args []string) error {
	// If output dir isn't specified, set to the current working dir
	var err error
	if genConfigCmdFlags.outputDir == "" {
		genConfigCmdFlags.outputDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working dir: %w", err)
		}
	}

	// Create dir path, ignoring "already exists" messages
	if err = os.MkdirAll(genConfigCmdFlags.outputDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	var genOptions []generate.GenOption //nolint:prealloc

	for _, registryMirror := range genConfigCmdFlags.registryMirrors {
		components := strings.SplitN(registryMirror, "=", 2)
		if len(components) != 2 {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		genOptions = append(genOptions, generate.WithRegistryMirror(components[0], components[1]))
	}

	if genConfigCmdFlags.talosVersion != "" {
		var versionContract *config.VersionContract

		versionContract, err = config.ParseContractFromVersion(genConfigCmdFlags.talosVersion)
		if err != nil {
			return fmt.Errorf("invalid talos-version: %w", err)
		}

		genOptions = append(genOptions, generate.WithVersionContract(versionContract))
	}

	genOptions = append(genOptions,
		generate.WithInstallDisk(genConfigCmdFlags.installDisk),
		generate.WithInstallImage(genConfigCmdFlags.installImage),
		generate.WithAdditionalSubjectAltNames(genConfigCmdFlags.additionalSANs),
		generate.WithDNSDomain(genConfigCmdFlags.dnsDomain),
		generate.WithPersist(genConfigCmdFlags.persistConfig),
	)

	commentsFlags := encoder.CommentsDisabled
	if genConfigCmdFlags.withDocs {
		commentsFlags |= encoder.CommentsDocs
	}

	if genConfigCmdFlags.withExamples {
		commentsFlags |= encoder.CommentsExamples
	}

	configBundle, err := GenV1Alpha1Config(
		genOptions,
		args[0],
		args[1],
		genConfigCmdFlags.kubernetesVersion,
		genConfigCmdFlags.configPatch,
		genConfigCmdFlags.configPatchControlPlane,
		genConfigCmdFlags.configPatchJoin)
	if err != nil {
		return err
	}

	if err = configBundle.Write(genConfigCmdFlags.outputDir, commentsFlags, machine.TypeInit, machine.TypeControlPlane, machine.TypeJoin); err != nil {
		return err
	}

	data, err := yaml.Marshal(configBundle.TalosConfig())
	if err != nil {
		return fmt.Errorf("failed to marshal config: %+v", err)
	}

	fullFilePath := filepath.Join(genConfigCmdFlags.outputDir, "talosconfig")

	if err = ioutil.WriteFile(fullFilePath, data, 0o644); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Printf("created %s\n", fullFilePath)

	return nil
}

func init() {
	genCmd.AddCommand(genConfigCmd)
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.installDisk, "install-disk", "/dev/sda", "the disk to install to")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.installImage, "install-image", helpers.DefaultImage(images.DefaultInstallerImageRepository), "the image used to perform an installation")
	genConfigCmd.Flags().StringSliceVar(&genConfigCmdFlags.additionalSANs, "additional-sans", []string{}, "additional Subject-Alt-Names for the APIServer certificate")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.dnsDomain, "dns-domain", "cluster.local", "the dns domain to use for cluster")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.configVersion, "version", "v1alpha1", "the desired machine config version to generate")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.talosVersion, "talos-version", "", "the desired Talos version to generate config for (backwards compatibility, e.g. v0.8)")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.kubernetesVersion, "kubernetes-version", "", "desired kubernetes version to run")
	genConfigCmd.Flags().StringVarP(&genConfigCmdFlags.outputDir, "output-dir", "o", "", "destination to output generated files")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.configPatch, "config-patch", "", "patch generated machineconfigs (applied to all node types)")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.configPatchControlPlane, "config-patch-control-plane", "", "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.configPatchJoin, "config-patch-join", "", "patch generated machineconfigs (applied to 'join' type)")
	genConfigCmd.Flags().StringSliceVar(&genConfigCmdFlags.registryMirrors, "registry-mirror", []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.persistConfig, "persist", "p", true, "the desired persist value for configs")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.withExamples, "with-examples", "", true, "renders all machine configs with the commented examples")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.withDocs, "with-docs", "", true, "renders all machine configs adding the documentation for each field")
}
