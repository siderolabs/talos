// genConfigCmd represents the gen config command.
var genConfigCmd = &cobra.Command{
	Use:   "config <cluster name> <cluster endpoint>",
	Short: "Generates a set of configuration files for Talos cluster",
	Long: `The cluster endpoint is the URL for the Kubernetes API. If you decide to use
a control plane node, common in a single node control plane setup, use port 6443 as
this is the port that the API server binds to on the control plane nodes
(e.g. https://1.2.3.4:6443).

When the configuration files are generated, the command will ask for the IP addresses
of the nodes in the cluster. The IP address is used to determine the node type.
The node types are as follows:

1. Init Node: The first node in the cluster. This node will bootstrap the cluster. There
   can only be one init node.
2. Control Plane Node: A node that hosts the control plane components. There can be
   multiple control plane nodes.
3. Worker Node: A node that hosts the worker components. There can be multiple worker
   nodes.

The IP address is also used to set the node's IP address in the configuration file.
You can use the IP address of the node or the IP address of the load balancer for the
node's type if you have a load balancer for the control plane nodes.
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate args
		if args[0] == "" {
			return fmt.Errorf("cluster name is required")
		}

		if args[1] == "" {
			return fmt.Errorf("cluster endpoint is required")
		}
		
		// Validate Talos version
		if err := config.ValidateTalosVersion(genConfigCmdFlags.GenOptions.TalosVersion); err != nil {
			return fmt.Errorf("invalid --talos-version: %w", err)
		}
		
		input, err := genConfigCmdFlags.GenConfigOptions(args)
		if err != nil {
			return fmt.Errorf("invalid generate configuration parameters: %w", err)
		}

		genConfig, err := generate.Config(input)
		if err != nil {
			return fmt.Errorf("failed to generate config: %w", err)
		}

		return genConfigCmdFlags.OutputGenerated(genConfig)
	},
}
