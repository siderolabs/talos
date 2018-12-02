// PACKET CREDENTIALS
// Your Packet API Key, grab one from the portal at 
// https://app.packet.net/portal#/api-keys
variable "packet_api_key" {
  default = "YOUR-API-KEY"
}

// Your Project ID, you can see it 
// here https://app.packet.net/portal#/projects/list/table
// variable "packet_project_id" {
//   default = "YOUR-PACKET-PROJECT-ID"
// }

// INFRASTRUCTURE
// The values for these variables can be found using the Packet API,
// more here https://www.packet.net/developers/api/.

// The Packet data center you would like to deploy into,
// the up-to-date list is available via the API endpoint /facilities
variable "packet_facility" {
  default = "ewr1"
}

// All server type slugs are available via the API endpoint /plans

// The Packet server type to use as your talos workers
variable "packet_agent_type" {
  default = "baremetal_0"
}

// The Packet server type to use as your talos masters
variable "packet_master_type"  {
  default = "baremetal_0"
}

// The Packet server type to use as your talos boot server
variable "packet_boot_type" {
  default = "baremetal_0"
}

 // How many DC/OS master servers would you like?
variable "talos_master_count" {
  default= "1"
}

// How many DC/OS private agent servers would you like?
variable "dcos_agent_count" {
  default = "2"
}

// How many DC/OS public agent servers would you like?
variable "dcos_public_agent_count" {
  default = "1"
}

// Github usernames to pull pub ssh keys for
variable "github_users" {
	default = []
}

variable "talos_version" {
  default = "v0.1.0-alpha.13"
}

variable "talos_boot_args" {
	default = [
	  "random.trust_cpu=on",
		"serial",
		"console=tty0",
		"console=ttyS1,115200n8",
		"ip=dhcp",
		"printk.devkmsg=on" 
	]
	// "talos.autonomy.io/userdata=http://147.75.198.255:8080/assets/talos/v0.1.0-alpha.13/userdata.yml",
	// "talos.autonomy.io/platform=bare-metal"
}

variable "talos_userdata_path" {
	default = ""
}

variable "talos_platform" {
	default = "bare-metal"
}
