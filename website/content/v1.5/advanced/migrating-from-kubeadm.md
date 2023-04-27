---
title: "Migrating from Kubeadm"
description: "Migrating Kubeadm-based clusters to Talos."
aliases:
  - ../guides/migrating-from-kubeadm
---

It is possible to migrate Talos from a cluster that is created using
[kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/) to Talos.

High-level steps are the following:

1. Collect CA certificates and a bootstrap token from a control plane node.
2. Create a Talos machine config with the CA certificates with the ones you collected.
3. Update control plane endpoint in the machine config to point to the existing control plane (i.e. your load balancer address).
4. Boot a new Talos machine and apply the machine config.
5. Verify that the new control plane node is ready.
6. Remove one of the old control plane nodes.
7. Repeat the same steps for all control plane nodes.
8. Verify that all control plane nodes are ready.
9. Repeat the same steps for all worker nodes, using the machine config generated for the workers.

## Remarks on kube-apiserver load balancer

While migrating to Talos, you need to make sure that your kube-apiserver load balancer is in place
and keeps pointing to the correct set of control plane nodes.

This process depends on your load balancer setup.

If you are using an LB that is external to the control plane nodes (e.g. cloud provider LB, F5 BIG-IP, etc.),
you need to make sure that you update the backend IPs of the load balancer to point to the control plane nodes as
you add Talos nodes and remove kubeadm-based ones.

If your load balancing is done on the control plane nodes (e.g. keepalived + haproxy on the control plane nodes),
you can do the following:

1. Add Talos nodes and remove kubeadm-based ones while updating the haproxy backends
   to point to the newly added nodes except the last kubeadm-based control plane node.
2. Turn off keepalived to drop the virtual IP used by the kubeadm-based nodes (introduces kube-apiserver downtime).
3. Set up a virtual-IP based new load balancer on the new set of Talos control plane nodes.
   Use the previous LB IP as the LB virtual IP.
4. Verify apiserver connectivity over the Talos-managed virtual IP.
5. Migrate the last control-plane node.

## Prerequisites

- Admin access to the kubeadm-based cluster
- Access to the `/etc/kubernetes/pki` directory (e.g. SSH & root permissions)
  on the control plane nodes of the kubeadm-based cluster
- Access to kube-apiserver load-balancer configuration

## Step-by-step guide

1. Download `/etc/kubernetes/pki` directory from a control plane node of the kubeadm-based cluster.

2. Create a new join token for the new control plane nodes:

   ```bash
   # inside a control plane node
   kubeadm token create
   ```

3. Create Talos secrets from the PKI directory you downloaded on step 1 and the token you generated on step 2:

   ```bash
   talosctl gen secrets --kubernetes-bootstrap-token <TOKEN> --from-kubernetes-pki <PKI_DIR>
   ```

4. Create a new Talos config from the secrets:

   ```bash
   talosctl gen config --with-secrets secrets.yaml <CLUSTER_NAME> https://<EXISTING_CLUSTER_LB_IP>
   ```

5. Collect the information about the kubeadm-based cluster from the kubeadm configmap:

   ```bash
   kubectl get configmap -n kube-system kubeadm-config -oyaml
   ```

   Take note of the following information in the `ClusterConfiguration`:
    - `.controlPlaneEndpoint`
    - `.networking.dnsDomain`
    - `.networking.podSubnet`
    - `.networking.serviceSubnet`

6. Replace the following information in the generated `controlplane.yaml`:
    - `.cluster.network.cni.name` with `none`
    - `.cluster.network.podSubnets[0]` with the value of the `networking.podSubnet` from the previous step
    - `.cluster.network.serviceSubnets[0]` with the value of the `networking.serviceSubnet` from the previous step
    - `.cluster.network.dnsDomain` with the value of the `networking.dnsDomain` from the previous step

7. Go through the rest of `controlplane.yaml` and `worker.yaml` to customize them according to your needs.

8. Bring up a Talos node to be the initial Talos control plane node.

9. Apply the generated `controlplane.yaml` to the Talos control plane node:

    ```bash
    talosctl --nodes <TALOS_NODE_IP> apply-config --insecure --file controlplane.yaml
    ```

10. Wait until the new control plane node joins the cluster and is ready.

    ```bash
    kubectl get node -owide --watch
    ```

11. Update your load balancer to point to the new control plane node.

12. Drain the old control plane node you are replacing:

    ```bash
    kubectl drain <OLD_NODE> --delete-emptydir-data --force --ignore-daemonsets --timeout=10m
    ```

13. Remove the old control plane node from the cluster:

    ```bash
    kubectl delete node <OLD_NODE>
    ```

14. Destroy the old node:

    ```bash
    # inside the node
    sudo kubeadm reset --force
    ```

15. Repeat the same steps, starting from step 7, for all control plane nodes.

16. Repeat the same steps, starting from step 7, for all worker nodes while applying the `worker.yaml` instead and skipping the LB step:

    ```bash
    talosctl --nodes <TALOS_NODE_IP> apply-config --insecure --file worker.yaml
    ```
