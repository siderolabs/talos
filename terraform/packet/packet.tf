provider "packet" {
  auth_token = "${var.packet_api_key}"
}

# Create a project
resource "packet_project" "talos" {
  name            = "talos"
  organization_id = "4cfa74b7-20f0-4e27-8c44-53f197a5b0e2"
}

data "http" "githubsshkey" {
  url = "https://github.com/${var.github_users[count.index]}.keys"
	count = "${length(var.github_users)}"
}

locals { 
	public_keys = "${compact(split("\n",join("\n",flatten(data.http.githubsshkey.*.body))))}"
}

resource "packet_ssh_key" "users" {
  name = "${format("user-%02d", count.index + 1)}"
	count = "${length(local.public_keys)}"
	public_key = "${local.public_keys[count.index]}"
}

// TODO:  look at assigning the masters private IPs
// resource "packet_reserved_ip_block" "private_elastic_ip" {
//     project_id = "${packet_project.myproject.id}"
//     quantity = (talos_master_master + talos_masters)
//     public = false
// }

data "template_file" "matchbox_profile" {
  template = "${file("${path.module}/templates/matchbox_profile.tmpl")}"

	  vars {
		    talos_version = "${var.talos_version}"
		    talos_boot_args = "${join("\n", var.talos_boot_args)}"
				talos_userdata = "talos.autonomy.io/userdata=${var.talos_userdata_path}"
				talos_platform = "talos.autonomy.io/platform=${var.talos_platform}"
		}
}

data "template_file" "matchbox_group" {
  template = "${file("${path.module}/templates/matchbox_group.tmpl")}"
}

resource "packet_device" "talos_bootstrap" {
  hostname         = "${format("talos-ipxe-%02d.example.com", count.index + 1)}"
  operating_system = "ubuntu_18_04"
  plan             = "${var.packet_boot_type}"
  facility         = "${var.packet_facility}"
  project_id       = "${packet_project.talos.id}"
  billing_cycle    = "hourly"

	// Install Matchbox
	provisioner "remote-exec" {
		inline = [
			"wget https://github.com/coreos/matchbox/releases/download/v0.7.1/matchbox-v0.7.1-linux-amd64.tar.gz",
			"tar xzvf matchbox-v0.7.1-linux-amd64.tar.gz",
			"mv matchbox-v0.7.1-linux-amd64/matchbox /usr/local/bin",
			"useradd -U matchbox",
			"mkdir -p /var/lib/matchbox/assets/talos/${var.talos_version}",
		  "mkdir -p /var/lib/matchbox/groups",
		  "mkdir -p /var/lib/matchbox/profiles",
			"chown -R matchbox:matchbox /var/lib/matchbox",
      "cp matchbox-v0.7.1-linux-amd64/contrib/systemd/matchbox-local.service /etc/systemd/system/matchbox.service",
      "systemctl daemon-reload",
      "systemctl enable matchbox",
      "systemctl start matchbox"
    ]
  }

	provisioner "file" {
	  content = "${data.template_file.matchbox_group.rendered}"
		destination = "/var/lib/matchbox/groups/talos.json"
	}
		
	provisioner "file" {
	  content = "${data.template_file.matchbox_profile.rendered}"
		destination = "/var/lib/matchbox/profiles/talos.json"
	}

	// Configure Matchbox
	provisioner "remote-exec" {
	  inline = [
		    "cd /var/lib/matchbox/assets/talos/${var.talos_version}",
  		  "wget --trust-server-names https://github.com/autonomy/talos/releases/download/${var.talos_version}/vmlinuz",
	  	  "wget --trust-server-names https://github.com/autonomy/talos/releases/download/${var.talos_version}/rootfs.tar.gz"
		]
	}
}

resource "packet_device" "talos_master" {
  hostname         = "${format("talosm-%02d.example.com", count.index + 1)}"
  operating_system = "custom_ipxe"
  plan             = "${var.packet_master_type}"
  count            = "${var.talos_master_count}"

	// TODO: need to see if we should use network.0 ( public ) or network.2 ( private )
  ipxe_script_url = "http://${packet_device.talos_bootstrap.network.2.address}:8080/boot.ipxe?profile=talos"

  facility         = "${var.packet_facility}"
  project_id       = "${packet_project.talos.id}"
  billing_cycle    = "hourly"
}

// resource "packet_device" "talos_agent" {
//   hostname         = "${format("talosw-%02d.example.com", count.index + 1)}"
//   operating_system = "custom_ipxe"
//   plan             = "${var.packet_agent_type}"
// 
//   count            = "${var.talos_agent_count}"
//   user_data        = "#cloud-config\n\nssh_authorized_keys:\n  - \"${file("${var.talos_ssh_public_key_path}")}\"\n"
//   facility         = "${var.packet_facility}"
//   project_id       = "${var.packet_project_id}"
//   billing_cycle    = "hourly"
// }
