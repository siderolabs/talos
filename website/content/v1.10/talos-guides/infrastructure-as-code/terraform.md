# How to Deploy Talos Cluster in Public Cloud Platforms

In his guide, we will learn how to deploy Talos resources by using Terraform with cloud platforms.

You can look at [terraform module docs](https://registry.terraform.io/providers/siderolabs/talos/latest/docs) which provide explanations on guides, resources and data sources as the latest version.

### How to Deploy Talos Cluster in AWS

#### Prerequisites

- AWS Account 
- AWS CLI
- Getting keys from AWS IAM and configuring it locally
- Terraform
- kubectl
- talosctl
- SSH key to connect to EC2 instances in talos (if necessary, use secrets manager to secure your secrets)

#### Setting up the environment

- Clone the [Sidero Labs' contribution repo](https://github.com/siderolabs/contrib) to your GitHub Account and then clone the repo on your computer.
- Then, write ```cd contrib``` to go to the directory
- Open up your code editor
- Go to aws file by writing ```cd examples/terraform/aws```
- In your terminal, write ```terraform init``` to initialize the Terraform file
- Then write ```terraform apply``` to deploy the resources, you may need to customize the command 
- After your resources were created in terraform, write ```kubectl apply -f manifests/ccm.yaml``` to deploy the manifest file

### How to Deploy Talos Cluster in Azure

#### Prerequisites

- Azure Account 
- Azure CLI
- Terraform
- kubectl
- talosctl

#### Setting up the environment

- Clone the [Sidero Labs' contribution repo](https://github.com/siderolabs/contrib) to your GitHub Account and then clone the repo on your computer.
- Then, write ```cd contrib``` to go to the directory
- Open up your code editor
- Go to aws file by writing ```cd examples/terraform/azure```
- In your terminal, write ```terraform init``` to initialize the Terraform file
- Then write ```terraform apply``` to deploy the resources, you may need to customize the command 

### How to Deploy Talos Cluster in GCP

#### Prerequisites

- GCP Account 
- gcloud CLI
- Terraform
- kubectl
- talosctl

#### Setting up the environment

- Clone the [Sidero Labs' contribution repo](https://github.com/siderolabs/contrib) to your GitHub Account and then clone the repo on your computer.
- Then, write ```cd contrib``` to go to the directory
- Open up your code editor
- Go to aws file by writing ```cd examples/terraform/gcp```
- In your terminal, write ```terraform init``` to initialize the Terraform file
- Then write ```terraform apply``` to deploy the resources, you may need to customize the command 

In addition to the steps in GCP, in Terraform, you need to define your project and your GCP account, and it is recommended to use secrets management tools