// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"
	sideronet "github.com/siderolabs/net"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	controlPlaneOutputType = "controlplane"
	workerOutputType       = "worker"
	talosconfigOutputType  = "talosconfig"

	stdoutOutput = "-"

	yamlExt = ".yaml"
)

var allOutputTypes = []string{
	controlPlaneOutputType,
	workerOutputType,
	talosconfigOutputType,
}

type configOutputPaths struct {
	controlPlane, worker, talosconfig string
}

var genConfigCmdFlags struct {
	additionalSANs    []string
	configVersion     string
	dnsDomain         string
	kubernetesVersion string
	talosVersion      string
	installDisk       string
	installImage      string

	// outputDir is a hidden flag kept for backwards compatibility
	outputDir string

	output                  string
	outputTypes             []string
	configPatch             []string
	configPatchControlPlane []string
	configPatchWorker       []string
	registryMirrors         []string
	persistConfig           bool
	withExamples            bool
	withDocs                bool
	withClusterDiscovery    bool
	withKubeSpan            bool
	withSecrets             string
}

// NewConfigCmd builds the config generation subcommand with the given name.
func NewConfigCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <cluster name> <cluster endpoint>", name),
		Short: "Generates a set of configuration files for Talos cluster",
		Long: `The cluster endpoint is the URL for the Kubernetes API. If you decide to use
a control plane node, common in a single node control plane setup, use port 6443 as
this is the port that the API server binds to on every control plane node. For an HA
setup, usually involving a load balancer, use the IP and port of the load balancer.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := validateClusterEndpoint(args[1])
			if err != nil {
				return err
			}

			switch genConfigCmdFlags.configVersion {
			case "v1alpha1":
				return writeConfig(args)
			default:
				return fmt.Errorf("unknown config version: %q", genConfigCmdFlags.configVersion)
			}
		},
	}
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

// GenerateConfigBundle generates the Talos config bundle
//
// GenerateConfigBundle is useful for integration with external tooling options.
func GenerateConfigBundle(genOptions []generate.Option,
	clusterName string,
	endpoint string,
	kubernetesVersion string,
	configPatch []string,
	configPatchControlPlane []string,
	configPatchWorker []string,
) (*bundle.Bundle, error) {
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

	addConfigPatch := func(configPatches []string, configOpt func([]configpatcher.Patch) bundle.Option) error {
		patches, err := configpatcher.LoadPatches(configPatches)
		if err != nil {
			return fmt.Errorf("error parsing config JSON patch: %w", err)
		}

		configBundleOpts = append(configBundleOpts, configOpt(patches))

		return nil
	}

	if err := addConfigPatch(configPatch, bundle.WithPatch); err != nil {
		return nil, err
	}

	if err := addConfigPatch(configPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return nil, err
	}

	if err := addConfigPatch(configPatchWorker, bundle.WithPatchWorker); err != nil {
		return nil, err
	}

	configBundle, err := bundle.NewBundle(configBundleOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate config bundle: %w", err)
	}

	return configBundle, nil
}

//nolint:gocyclo
func writeConfig(args []string) error {
	if err := validateFlags(); err != nil {
		return err
	}

	paths, err := outputPaths()
	if err != nil {
		return err
	}

	var genOptions []generate.Option //nolint:prealloc

	for _, registryMirror := range genConfigCmdFlags.registryMirrors {
		left, right, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		genOptions = append(genOptions, generate.WithRegistryMirror(left, right))
	}

	if genConfigCmdFlags.talosVersion != "" {
		var versionContract *config.VersionContract

		versionContract, err = config.ParseContractFromVersion(genConfigCmdFlags.talosVersion)
		if err != nil {
			return fmt.Errorf("invalid talos-version: %w", err)
		}

		genOptions = append(genOptions, generate.WithVersionContract(versionContract))
	}

	if genConfigCmdFlags.withKubeSpan {
		genOptions = append(genOptions,
			generate.WithNetworkOptions(
				v1alpha1.WithKubeSpan(),
			),
		)
	}

	if genConfigCmdFlags.withSecrets != "" {
		var secretsBundle *secrets.Bundle

		secretsBundle, err = secrets.LoadBundle(genConfigCmdFlags.withSecrets)
		if err != nil {
			return fmt.Errorf("failed to load secrets bundle: %w", err)
		}

		genOptions = append(genOptions, generate.WithSecretsBundle(secretsBundle))
	}

	genOptions = append(genOptions,
		generate.WithInstallDisk(genConfigCmdFlags.installDisk),
		generate.WithInstallImage(genConfigCmdFlags.installImage),
		generate.WithAdditionalSubjectAltNames(genConfigCmdFlags.additionalSANs),
		generate.WithDNSDomain(genConfigCmdFlags.dnsDomain),
		generate.WithPersist(genConfigCmdFlags.persistConfig),
		generate.WithClusterDiscovery(genConfigCmdFlags.withClusterDiscovery),
	)

	commentsFlags := encoder.CommentsDisabled
	if genConfigCmdFlags.withDocs {
		commentsFlags |= encoder.CommentsDocs
	}

	if genConfigCmdFlags.withExamples {
		commentsFlags |= encoder.CommentsExamples
	}

	configBundle, err := GenerateConfigBundle(
		genOptions,
		args[0],
		args[1],
		genConfigCmdFlags.kubernetesVersion,
		genConfigCmdFlags.configPatch,
		genConfigCmdFlags.configPatchControlPlane,
		genConfigCmdFlags.configPatchWorker)
	if err != nil {
		return err
	}

	return writeConfigBundle(configBundle, paths, commentsFlags)
}

func validateFlags() error {
	if len(genConfigCmdFlags.outputTypes) == 0 {
		return errors.New("at least one output type must be specified")
	}

	if len(genConfigCmdFlags.outputTypes) > 1 && genConfigCmdFlags.output == stdoutOutput {
		return errors.New("can't use multiple output types with stdout")
	}

	if genConfigCmdFlags.outputDir != "" && genConfigCmdFlags.output != "" {
		return errors.New("can't use both output-dir and output")
	}

	if genConfigCmdFlags.outputDir != "" {
		genConfigCmdFlags.output = genConfigCmdFlags.outputDir
	}

	var err error

	for _, outputType := range genConfigCmdFlags.outputTypes {
		if !slices.ContainsFunc(allOutputTypes, func(t string) bool {
			return t == outputType
		}) {
			err = multierror.Append(err, fmt.Errorf("invalid output type: %q", outputType))
		}
	}

	return err
}

func writeConfigBundle(configBundle *bundle.Bundle, outputPaths configOutputPaths, commentsFlags encoder.CommentsFlags) error {
	outputTypesSet := xslices.ToSet(genConfigCmdFlags.outputTypes)

	if _, ok := outputTypesSet[controlPlaneOutputType]; ok {
		data, err := configBundle.Serialize(commentsFlags, machine.TypeControlPlane)
		if err != nil {
			return err
		}

		if err = writeToDestination(data, outputPaths.controlPlane, 0o644); err != nil {
			return err
		}
	}

	if _, ok := outputTypesSet[workerOutputType]; ok {
		data, err := configBundle.Serialize(commentsFlags, machine.TypeWorker)
		if err != nil {
			return err
		}

		if err = writeToDestination(data, outputPaths.worker, 0o644); err != nil {
			return err
		}
	}

	if _, ok := outputTypesSet[talosconfigOutputType]; ok {
		data, err := yaml.Marshal(configBundle.TalosConfig())
		if err != nil {
			return fmt.Errorf("failed to marshal config: %+v", err)
		}

		if err = writeToDestination(data, outputPaths.talosconfig, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func writeToDestination(data []byte, destination string, permissions os.FileMode) error {
	if destination == stdoutOutput {
		_, err := os.Stdout.Write(data)

		return err
	}

	if err := validateFileExists(destination); err != nil {
		return err
	}

	parentDir := filepath.Dir(destination)

	// Create dir path, ignoring "already exists" messages
	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	err := os.WriteFile(destination, data, permissions)

	fmt.Fprintf(os.Stderr, "Created %s\n", destination)

	return err
}

func outputPaths() (configOutputPaths, error) {
	// output to stdout
	if genConfigCmdFlags.output == stdoutOutput {
		return configOutputPaths{controlPlane: stdoutOutput, worker: stdoutOutput, talosconfig: stdoutOutput}, nil
	}

	// output is not specified - use current working directory as the default
	if genConfigCmdFlags.output == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return configOutputPaths{}, err
		}

		controlPlane := filepath.Join(cwd, machine.TypeControlPlane.String()+yamlExt)
		worker := filepath.Join(cwd, machine.TypeWorker.String()+yamlExt)
		talosconfig := filepath.Join(cwd, "talosconfig")

		return configOutputPaths{controlPlane: controlPlane, worker: worker, talosconfig: talosconfig}, nil
	}

	// output is specified

	// if a single output type is specified, treat --output as a file path and not a directory
	// except when the deprecated flag of --output-dir is specified - it is always treated as a directory
	if len(genConfigCmdFlags.outputTypes) == 1 && genConfigCmdFlags.outputDir == "" { // specified output is a file
		return configOutputPaths{
			controlPlane: genConfigCmdFlags.output,
			worker:       genConfigCmdFlags.output,
			talosconfig:  genConfigCmdFlags.output,
		}, nil
	}

	// treat --output as a directory
	controlPlane := filepath.Join(genConfigCmdFlags.output, machine.TypeControlPlane.String()+yamlExt)
	worker := filepath.Join(genConfigCmdFlags.output, machine.TypeWorker.String()+yamlExt)
	talosconfig := filepath.Join(genConfigCmdFlags.output, "talosconfig")

	return configOutputPaths{controlPlane: controlPlane, worker: worker, talosconfig: talosconfig}, nil
}

func validateClusterEndpoint(endpoint string) error {
	// Validate url input to ensure it has https:// scheme before we attempt to gen
	u, err := url.Parse(endpoint)
	if err != nil {
		if !strings.Contains(endpoint, "/") {
			// not a URL, could be just host:port
			u = &url.URL{
				Host: endpoint,
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

	if err = sideronet.ValidateEndpointURI(endpoint); err != nil {
		return fmt.Errorf("error validating the cluster endpoint URL: %w", err)
	}

	return nil
}

func init() {
	genConfigCmd := NewConfigCmd("config")

	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.installDisk, "install-disk", "/dev/sda", "the disk to install to")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.installImage, "install-image", helpers.DefaultImage(images.DefaultInstallerImageRepository), "the image used to perform an installation")
	genConfigCmd.Flags().StringSliceVar(&genConfigCmdFlags.additionalSANs, "additional-sans", []string{}, "additional Subject-Alt-Names for the APIServer certificate")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.dnsDomain, "dns-domain", "cluster.local", "the dns domain to use for cluster")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.configVersion, "version", "v1alpha1", "the desired machine config version to generate")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.talosVersion, "talos-version", "", "the desired Talos version to generate config for (backwards compatibility, e.g. v0.8)")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	genConfigCmd.Flags().StringArrayVar(&genConfigCmdFlags.configPatch, "config-patch", nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
	genConfigCmd.Flags().StringArrayVar(&genConfigCmdFlags.configPatchControlPlane, "config-patch-control-plane", nil, "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
	genConfigCmd.Flags().StringArrayVar(&genConfigCmdFlags.configPatchWorker, "config-patch-worker", nil, "patch generated machineconfigs (applied to 'worker' type)")
	genConfigCmd.Flags().StringSliceVar(&genConfigCmdFlags.registryMirrors, "registry-mirror", []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.persistConfig, "persist", "p", true, "the desired persist value for configs")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.withExamples, "with-examples", "", true, "renders all machine configs with the commented examples")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.withDocs, "with-docs", "", true, "renders all machine configs adding the documentation for each field")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.withClusterDiscovery, "with-cluster-discovery", "", true, "enable cluster discovery feature")
	genConfigCmd.Flags().BoolVarP(&genConfigCmdFlags.withKubeSpan, "with-kubespan", "", false, "enable KubeSpan feature")
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.withSecrets, "with-secrets", "", "use a secrets file generated using 'gen secrets'")

	genConfigCmd.Flags().StringSliceVarP(&genConfigCmdFlags.outputTypes, "output-types", "t", allOutputTypes, fmt.Sprintf("types of outputs to be generated. valid types are: %q", allOutputTypes))
	genConfigCmd.Flags().StringVarP(&genConfigCmdFlags.output, "output", "o", "",
		`destination to output generated files. when multiple output types are specified, it must be a directory. for a single output type, it must either be a file path, or "-" for stdout`)
	genConfigCmd.Flags().StringVar(&genConfigCmdFlags.outputDir, "output-dir", "", "destination to output generated files") // kept for backwards compatibility
	genConfigCmd.Flags().MarkHidden("output-dir")                                                                           //nolint:errcheck

	Cmd.AddCommand(genConfigCmd)
}
