// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	genv1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	configVersion     string
	kubernetesVersion string
	installDisk       string
	installImage      string
	outputDir         string
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration",
	Long:  ``,
}

// configEndpointCmd represents the config endpoint command.
var configEndpointCmd = &cobra.Command{
	Use:     "endpoint <endpoint>...",
	Aliases: []string{"endpoints"},
	Short:   "Set the endpoint(s) for the current context",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		if c.Context == "" {
			helpers.Fatalf("no context is set")
		}

		c.Contexts[c.Context].Endpoints = args
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configNodeCmd represents the config node command.
var configNodeCmd = &cobra.Command{
	Use:     "node <endpoint>...",
	Aliases: []string{"nodes"},
	Short:   "Set the node(s) for the current context",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		if c.Context == "" {
			helpers.Fatalf("no context is set")
		}

		c.Contexts[c.Context].Nodes = args
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configContextCmd represents the configc context command.
var configContextCmd = &cobra.Command{
	Use:   "context <context>",
	Short: "Set the current context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}
		context := args[0]
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		c.Context = context
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configAddCmd represents the config add command.
var configAddCmd = &cobra.Command{
	Use:   "add <context>",
	Short: "Add a new context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}
		context := args[0]
		c, err := config.Open(talosconfig)
		if err != nil {
			helpers.Fatalf("error reading config: %s", err)
		}
		caBytes, err := ioutil.ReadFile(ca)
		if err != nil {
			helpers.Fatalf("error reading CA: %s", err)
		}
		crtBytes, err := ioutil.ReadFile(crt)
		if err != nil {
			helpers.Fatalf("error reading certificate: %s", err)
		}
		keyBytes, err := ioutil.ReadFile(key)
		if err != nil {
			helpers.Fatalf("error reading key: %s", err)
		}
		newContext := &config.Context{
			CA:  base64.StdEncoding.EncodeToString(caBytes),
			Crt: base64.StdEncoding.EncodeToString(crtBytes),
			Key: base64.StdEncoding.EncodeToString(keyBytes),
		}
		if c.Contexts == nil {
			c.Contexts = map[string]*config.Context{}
		}
		c.Contexts[context] = newContext
		if err := c.Save(talosconfig); err != nil {
			helpers.Fatalf("error writing config: %s", err)
		}
	},
}

// configGenerateCmd represents the config generate command.
var configGenerateCmd = &cobra.Command{
	Use:   "generate <cluster name> https://<load balancer IP or DNS name>",
	Short: "Generate a set of configuration files",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			log.Fatal("expected a cluster name and load balancer IP or DNS name")
		}

		// Validate url input to ensure it has https:// scheme before we attempt to gen
		u, err := url.Parse(args[1])
		if err != nil {
			helpers.Fatalf("failed to parse load balancer IP or DNS name: %s", err)
		}
		if u.Scheme == "" {
			helpers.Fatalf("no scheme specified for load balancer IP or DNS name\ntry \"https://<load balancer IP or DNS name>\"")
		}

		switch configVersion {
		case "v1alpha1":
			genV1Alpha1Config(args)
		}
	},
}

func genV1Alpha1Config(args []string) {
	// If output dir isn't specified, set to the current working dir
	var err error
	if outputDir == "" {
		outputDir, err = os.Getwd()
		if err != nil {
			helpers.Fatalf("failed to get working dir: %w", err)
		}
	}

	// Create dir path, ignoring "already exists" messages
	if err = os.MkdirAll(outputDir, os.ModePerm); err != nil && !os.IsExist(err) {
		helpers.Fatalf("failed to create output dir: %w", err)
	}

	input, err := genv1alpha1.NewInput(args[0], args[1], kubernetesVersion)
	if err != nil {
		helpers.Fatalf("failed to generate PKI and tokens: %w", err)
	}

	input.AdditionalSubjectAltNames = additionalSANs
	input.InstallDisk = installDisk
	input.InstallImage = installImage

	for _, t := range []genv1alpha1.Type{genv1alpha1.TypeInit, genv1alpha1.TypeControlPlane, genv1alpha1.TypeJoin} {
		if err = writeV1Alpha1Config(input, t, t.String()); err != nil {
			helpers.Fatalf("failed to generate config for %s: %w", t.String(), err)
		}
	}

	newConfig := &config.Config{
		Context: input.ClusterName,
		Contexts: map[string]*config.Context{
			input.ClusterName: {
				Endpoints: []string{"127.0.0.1"},
				CA:        base64.StdEncoding.EncodeToString(input.Certs.OS.Crt),
				Crt:       base64.StdEncoding.EncodeToString(input.Certs.Admin.Crt),
				Key:       base64.StdEncoding.EncodeToString(input.Certs.Admin.Key),
			},
		},
	}

	data, err := yaml.Marshal(newConfig)
	if err != nil {
		helpers.Fatalf("failed to marshal config: %+v", err)
	}

	fullFilePath := filepath.Join(outputDir, "talosconfig")

	if err = ioutil.WriteFile(fullFilePath, data, 0644); err != nil {
		helpers.Fatalf("%w", err)
	}

	fmt.Printf("created %s\n", fullFilePath)
}

func writeV1Alpha1Config(input *genv1alpha1.Input, t genv1alpha1.Type, name string) (err error) {
	var data string

	data, err = genv1alpha1.Config(t, input)
	if err != nil {
		return err
	}

	name = strings.ToLower(name) + ".yaml"
	fullFilePath := filepath.Join(outputDir, name)

	if err = ioutil.WriteFile(fullFilePath, []byte(data), 0644); err != nil {
		return err
	}

	fmt.Printf("created %s\n", fullFilePath)

	return nil
}

func init() {
	configCmd.AddCommand(configContextCmd, configEndpointCmd, configNodeCmd, configAddCmd, configGenerateCmd)
	configAddCmd.Flags().StringVar(&ca, "ca", "", "the path to the CA certificate")
	configAddCmd.Flags().StringVar(&crt, "crt", "", "the path to the certificate")
	configAddCmd.Flags().StringVar(&key, "key", "", "the path to the key")
	configGenerateCmd.Flags().StringVar(&installDisk, "install-disk", "/dev/sda", "the disk to install to")
	configGenerateCmd.Flags().StringVar(&installImage, "install-image", fmt.Sprintf("%s:%s", constants.DefaultInstallerImageRepository, version.Tag), "the image used to perform an installation")
	configGenerateCmd.Flags().StringSliceVar(&additionalSANs, "additional-sans", []string{}, "additional Subject-Alt-Names for the APIServer certificate")
	configGenerateCmd.Flags().StringVar(&configVersion, "version", "v1alpha1", "the desired machine config version to generate")
	configGenerateCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	configGenerateCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "destination to output generated files")
	helpers.Should(configAddCmd.MarkFlagRequired("ca"))
	helpers.Should(configAddCmd.MarkFlagRequired("crt"))
	helpers.Should(configAddCmd.MarkFlagRequired("key"))
	rootCmd.AddCommand(configCmd)
}
