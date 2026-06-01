{
    "cluster_name": .cluster_name,
    "talos_version_contract": .talos_version_contract,
    "kubernetes_version": .kubernetes_version,
    "azure_location": "eastus",
    "control_plane": {
        "vm_os_id": .vm_os_id,
        "vm_size": "Standard_B2s"
    },
    "worker_groups": [
        {
            "name": "default",
            "vm_os_id": .vm_os_id,
            "vm_size": "Standard_B2s"
        }
    ],
    "extra_tags": {
        "Cluster Name": .cluster_name,
        "Project": "talos-e2e-ci",
        "Environment": "ci"
    }
}
