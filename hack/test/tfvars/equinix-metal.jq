{
    "cluster_name": .cluster_name,
    "em_api_token": .em_api_token,
    "talos_version_contract": .talos_version_contract,
    "kubernetes_version": .kubernetes_version,
    "em_region": .em_region,
    "em_project_id": .em_project_id,
    "control_plane": {
        "num_instances": 1,
        "plan": "c3.small.x86",
        "ipxe_script_url": .ipxe_script_url_amd64,
        "install_image": .install_image
    },
    "worker_groups": [
        {
            "name": "amd64",
            "num_instances": 1,
            "plan": "c3.small.x86",
            "ipxe_script_url": .ipxe_script_url_amd64,
            "install_image": .install_image
        },
        {
            "name": "arm64",
            "plan": "c3.large.arm64",
            "num_instances": 1,
            "ipxe_script_url": .ipxe_script_url_arm64,
            "install_image": .install_image,
            "config_patch_files": [
                "equinix-arm64.yaml"
            ]
        }
    ],
    "extra_tags": [
        "Cluster Name=" + .cluster_name,
        "Project=talos-e2e-ci",
        "Environment=ci"
    ]
}
