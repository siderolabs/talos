package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

// injectCmd represents the inject command
var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Inject data into fields in the user data.",
	Long:  ``,
}

// injectOSCmd represents the gen inject os command
var injectOSCmd = &cobra.Command{
	Use:   "os",
	Short: "Populates fields in the user data that are generated for the OS",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		var err error

		if len(args) != 1 {
			os.Exit(1)
		}
		filename := args[0]
		fileBytes, err := ioutil.ReadFile(filename)
		if err != nil {
			os.Exit(1)
		}
		data := &userdata.UserData{}
		if err = yaml.Unmarshal(fileBytes, data); err != nil {
			os.Exit(1)
		}
		if data.OS.Security == nil {
			data.OS.Security = &userdata.Security{}
			data.OS.Security.Identity = &userdata.CertificateAndKeyPaths{}
			data.OS.Security.CA = &userdata.CertificateAndKeyPaths{}
		}
		if identity != "" {
			fileBytes, err = ioutil.ReadFile(identity + ".crt")
			if err != nil {
				os.Exit(1)
			}
			data.OS.Security.Identity.Crt = base64.StdEncoding.EncodeToString(fileBytes)
			fileBytes, err = ioutil.ReadFile(identity + ".key")
			if err != nil {
				os.Exit(1)
			}
			data.OS.Security.Identity.Key = base64.StdEncoding.EncodeToString(fileBytes)
		}
		if ca != "" {
			fileBytes, err = ioutil.ReadFile(ca + ".crt")
			if err != nil {
				os.Exit(1)
			}
			data.OS.Security.CA.Crt = base64.StdEncoding.EncodeToString(fileBytes)
		}

		dataBytes, err := yaml.Marshal(data)
		if err != nil {
			os.Exit(1)
		}
		if err := ioutil.WriteFile(filename, dataBytes, 0700); err != nil {
			os.Exit(1)
		}
	},
}

// injectKubernetesCmd represents the gen inject kubernetes command
var injectKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Populates fields in the user data that are generated for Kubernetes",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			os.Exit(1)
		}
		filename := args[0]
		fileBytes, err := ioutil.ReadFile(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		data := &userdata.UserData{}
		if err = yaml.Unmarshal(fileBytes, data); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if data.Kubernetes.CA == nil {
			data.Kubernetes.CA = &userdata.CertificateAndKeyPaths{}
		}
		if ca != "" {
			fileBytes, err = ioutil.ReadFile(ca + ".crt")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			data.Kubernetes.CA.Crt = base64.StdEncoding.EncodeToString(fileBytes)
			fileBytes, err = ioutil.ReadFile(ca + ".key")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			data.Kubernetes.CA.Key = base64.StdEncoding.EncodeToString(fileBytes)
		}
		if hash != "" {
			fileBytes, err = ioutil.ReadFile(hash + ".sha256")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			data.Kubernetes.DiscoveryTokenCACertHashes = []string{string(fileBytes)}
		}

		dataBytes, err := yaml.Marshal(data)
		if err != nil {
			os.Exit(1)
		}
		if err := ioutil.WriteFile(filename, dataBytes, 0700); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	// Inject OS
	injectOSCmd.Flags().StringVar(&ca, "ca", "", "the basename of the key pair to use as the CA")
	injectOSCmd.Flags().StringVar(&identity, "identity", "", "the basename of the key pair to use as the identity")
	// Inject Kubernetes
	injectKubernetesCmd.Flags().StringVar(&ca, "ca", "", "the basename of the key pair to use as the CA")
	injectKubernetesCmd.Flags().StringVar(&hash, "hash", "", "the basename of the CA to use as the hash")

	injectCmd.AddCommand(injectOSCmd, injectKubernetesCmd)
	rootCmd.AddCommand(injectCmd)
}
