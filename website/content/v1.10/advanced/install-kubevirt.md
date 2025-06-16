---
title: "Install KubeVirt on Talos"
description: "This is a guide on how to get started with KubeVirt on Talos"
---

KubeVirt allows you to run virtual machines on Kubernetes.
It runs with QEMU and KVM to provide a seamless virtual machine experience and can be mixed with containerized workloads.
This guide explains on how to install KubeVirt on Talos.

## Prerequisites

For KubeVirt and Talos to work you have to enable certain configurations in the BIOS and configure Talos properly for it to work.

### Enable virtualization in your BIOS

On many new PCs and servers, virtualization is enabled by default.
Please consult your manufacturer on how to enable this in the BIOS.
You can also run KubeVirt from within a virtual machine.
For that to work you have to enable Nested Virtualization.
This can also be done in the BIOS.

### Configure your network interface in bridge mode (optional)

When you want to leverage [Multus]({{< relref "../kubernetes-guides/network/multus" >}}) to give your virtual machines direct access to your node network, your bridge needs to be configured properly.
This can be done by setting your network interface in bridge mode.
You can look up the network interface name by using the following command:

```bash
$ talosctl get links -n 10.99.101.9
NODE          NAMESPACE   TYPE         ID             VERSION   TYPE       KIND     HW ADDR                                           OPER STATE   LINK STATE
10.99.101.9   network     LinkStatus   bond0          1         ether      bond     52:62:01:53:5b:a7                                 down         false
10.99.101.9   network     LinkStatus   br0            3         ether      bridge   bc:24:11:a1:98:fc                                 up           true
10.99.101.9   network     LinkStatus   cni0           9         ether      bridge   1e:5e:99:8f:1e:19                                 up           true
10.99.101.9   network     LinkStatus   dummy0         1         ether      dummy    62:1c:3e:d5:72:11                                 down         false
10.99.101.9   network     LinkStatus   eth0           5         ether               bc:24:11:a1:98:fc
```

In this case, this network interface is called `eth0`.
Now you can configure your bridge properly.
This can be done in the machine config of your node:

```yaml
machine:
  network:
      interfaces:
      - interface: br0
        addresses:
          - 10.99.101.9/24
        bridge:
          stp:
            enabled: true
          interfaces:
              - eth0 # This must be changed to your matching interface name
        routes:
            - network: 0.0.0.0/0 # The route's network (destination).
              gateway: 10.99.101.254 # The route's gateway (if empty, creates link scope route).
              metric: 1024 # The optional metric for the route.
```

### Install the `local-path-provisioner`

When we are using KubeVirt, we are also installing the CDI (containerized data importer) operator.
For this to work properly, we have to install the `local-path-provisioner`.
This CNI can be used to write scratch space when importing images with the CDI.

You can install the `local-path-provisioner` by following [this guide]({{< relref "../kubernetes-guides/configuration/local-storage" >}}).

### Configure storage

If you would like to use features such as `LiveMigration` shared storage is neccesary.
You can either choose to install a CSI that connects to NFS or you can install Longhorn, for example.
For more information on how to install Longhorn on Talos you can follow [this](https://longhorn.io/docs/1.7.2/advanced-resources/os-distro-specific/talos-linux-support/) link.

To install the NFS-CSI driver, you can follow [This](https://github.com/kubernetes-csi/csi-driver-nfs/blob/master/docs/install-csi-driver-v4.9.0.md) guide.

After the installation of the NFS-CSI driver is done, you can create a storage class for the NFS CSI driver to work:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nfs-csi
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: nfs.csi.k8s.io
parameters:
  server: 10.99.102.253
  share: /mnt/data/nfs/kubernetes_csi
reclaimPolicy: Delete
volumeBindingMode: Immediate
mountOptions:
  - nfsvers=3
  - nolock
```

Note that this is just an example.
Make sure to set the `nolock` option.
If not, the nfs-csi storageclass won't work, because talos doesn't have a `rpc.statd` daemon running.

### Install `virtctl`

`virtctl` is needed for communication between the CLI and the KubeVirt api server.

You can install the `virtctl` client directly by running:

```bash
export VERSION=$(curl https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirt/stable.txt)
wget https://github.com/kubevirt/kubevirt/releases/download/${VERSION}/virtctl-${VERSION}-linux-amd64
```

Or you can use [krew](https://github.com/kubernetes-sigs/krew/#installation) to integrate it nicely in `kubectl`:

```bash
kubectl krew install virt
```

## Installing KubeVirt

After the neccesary preperations are done, you can now install KubeVirt.
This can either be done through the [Operator Lifecycle Manager](https://olm.operatorframework.io/docs/getting-started/) or by just simply applying a YAML file.
We will keep this simple and do the following:

```bash
# Point at latest release
export RELEASE=$(curl https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirt/stable.txt)
# Deploy the KubeVirt operator
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${RELEASE}/kubevirt-operator.yaml
```

After the operator is installed, it is time to apply the Custom Resource (CR) for the operator to fully deploy KubeVirt.

```yaml
---
apiVersion: kubevirt.io/v1
kind: KubeVirt
metadata:
  name: kubevirt
  namespace: kubevirt
spec:
  configuration:
    developerConfiguration:
      featureGates:
        - LiveMigration
        - NetworkBindingPlugins
    smbios:
      sku: "TalosCloud"
      version: "v0.1.0"
      manufacturer: "Talos Virtualization"
      product: "talosvm"
      family: "ccio"
  workloadUpdateStrategy:
    workloadUpdateMethods:
    - LiveMigrate # enable if you have deployed either Longhorn or NFS-CSI for shared storage.
```

### KubeVirt configuration options

In this yaml file we specified certain configurations:

#### `featureGates`

KubeVirt has a set of features that are not mature enough to be enabled by default.
As such, they are protected by a Kubernetes concept called feature gates.
More information about the feature gates can be found in the [KubeVirt](https://kubevirt.io/user-guide/cluster_admin/activating_feature_gates/) documentation.

In this example we enable:

- `LiveMigration` -- For live migration of virtual machines to other nodes
- `NetworkBindingPlugins` -- This is needed for Multus to work.

#### `smbios`

Here we configure a specific smbios configuration.
This can be useful when you want to give your virtual machines a own sku, manufacturer name etc.

#### `workloadUpdateStrategy`

If this is configured, virtual machines will be live migrated to other nodes when KubeVirt is updated.

## Installing CDI

The CDI (containerized data importer) is needed to import virtual disk images in your KubeVirt cluster.
The CDI can do the following:

- Import images of type:
  - qcow2
  - raw
  - iso
- Import disks from either:
  - http/https
  - uploaded through virtctl
  - Container registry
  - Another PVC

You can either import these images by creating a DataVolume CR or by integrating this in your `VirtualMachine` CR.

When applying either the `DataVolume` CR or the `VirtualMachine` CR with a `dataVolumeTemplates`, the CDI kicks in and will do the following:

- creates a PVC with the requirements from either the `DataVolume` or the `dataVolumeTemplates`
- starts a pod
- writes temporary scratch space to local disk
- downloads the image
- extracts it to the temporary scratch space
- copies the image to the PVC

Installing the CDI is very simple:

```bash
# Point to latest release
export TAG=$(curl -s -w %{redirect_url} \
https://github.com/kubevirt/containerized-data-importer/releases/latest)

export VERSION=$(echo ${TAG##*/})

# install operator
kubectl create -f \
https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-operator.yaml
```

After that, you can apply a CDI CR for the CDI operator to fully deploy CDI:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDI
metadata:
  name: cdi
spec:
  config:
    scratchSpaceStorageClass: local-path
    podResourceRequirements:
      requests:
        cpu: "100m"
        memory: "60M"
      limits:
        cpu: "750m"
        memory: "2Gi"
```

This CR has some special settings that are needed for CDI to work properly:

### `scratchSpaceStorageClass`

This is the storage class that we installed earlier with the `local-path-provisioner`.
This is needed for the CDI to write scratch space to local disk before importing the image

### `podResourceRequirements`

In many cases the default resource requests and limits are not sufficient for the importer pod to import the image.
This will result in a crash of the importer pod.

After applying this yaml file, the CDI operator is ready.

## Creating your first virtual machine

Now it is time to create your first virtual machine in KubeVirt.
Below we will describe two examples:

- A virtual machine with the default CNI
- A virtual machine with Multus

### Basic virtual machine example with default CNI

```yaml
---
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: fedora-vm
spec:
  running: false
  template:
    metadata:
      labels:
        kubevirt.io/vm: fedora-vm
      annotations:
        kubevirt.io/allow-pod-bridge-network-live-migration: "true"

    spec:
      evictionStrategy: LiveMigrate
      domain:
        cpu:
          cores: 2
        resources:
          requests:
            memory: 4G
        devices:
          disks:
            - name: fedora-vm-pvc
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: podnet
            masquerade: {}
      networks:
        - name: podnet
          pod: {}
      volumes:
        - name: fedora-vm-pvc
          persistentVolumeClaim:
            claimName: fedora-vm-pvc
        - name: cloudinitdisk
          cloudInitNoCloud:
            networkData: |
              network:
                version: 1
                config:
                  - type: physical
                    name: eth0
                    subnets:
                      - type: dhcp
            userData: |-
              #cloud-config
              users:
                - name: cloud-user
                  ssh_authorized_keys:
                    - ssh-rsa ....
                  sudo: ['ALL=(ALL) NOPASSWD:ALL']
                  groups: sudo
                  shell: /bin/bash
              runcmd:
                - "sudo touch /root/installed"
                - "sudo dnf update"
                - "sudo dnf install httpd fastfetch -y"
                - "sudo systemctl daemon-reload"
                - "sudo systemctl enable httpd"
                - "sudo systemctl start --no-block httpd"

  dataVolumeTemplates:
  - metadata:
      name: fedora-vm-pvc
    spec:
      storage:
        resources:
          requests:
            storage: 35Gi
        accessModes:
          - ReadWriteMany
        storageClassName: "nfs-csi"
      source:
        http:
          url: "https://fedora.mirror.wearetriple.com/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2"
```

In this examples we install a basic Fedora 40 virtual machine and install a webserver.

After applying this YAML, the CDI will import the image and create a `Datavolume`.
You can monitor this process by running:

```bash
kubectl get dv -w
```

After the `DataVolume` is created, you can start the virtual machine:

```bash
kubectl virt start fedora-vm
```

By starting the virtual machine, KubeVirt will create a instance of that `VirtualMachine` called `VirtualMachineInstance`:

```bash
kubectl get virtualmachineinstance
NAME        AGE   PHASE     IP            NODENAME   READY
fedora-vm   13s   Running   10.244.4.92   kube1      True
```

You can view the console of the virtual machine by running:

```bash
kubectl virt console fedora-vm
```

or by running:

```bash
kubectl virt vnc fedora-vm
```

with the `console` command it will open a terminal to the virtual machine.
With the `vnc` command, it will open `vncviewer`.
Note that a `vncviewer` needs to installed for it to work.

Now you can create a `Service` object to expose the virtual machine to the outside.
In this example we will use [MetalLB](https://metallb.universe.tf/) as a LoadBalancer.

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    kubevirt.io/vm: fedora-vm
  name: fedora-vm
spec:
  ipFamilyPolicy: PreferDualStack
  externalTrafficPolicy: Local
  ports:
  - name: ssh
    port: 22
    protocol: TCP
    targetPort: 22
  - name: httpd
    port: 80
    protocol: TCP
    targetPort: 80
  selector:
    kubevirt.io/vm: fedora-vm
  type: LoadBalancer
```

```bash
$ kubectl get svc
NAME             TYPE           CLUSTER-IP     EXTERNAL-IP                        PORT(S)                     AGE
fedora-vm        LoadBalancer   10.96.14.253   10.99.50.1                         22:31149/TCP,80:31445/TCP   2s
```

And we can reach the server with either ssh or http:

```bash
$ nc -zv 10.99.50.1 22
Ncat: Version 7.92 ( https://nmap.org/ncat )
Ncat: Connected to 10.99.50.1:22.
Ncat: 0 bytes sent, 0 bytes received in 0.01 seconds.

$ nc -zv 10.99.50.1 80
Ncat: Version 7.92 ( https://nmap.org/ncat )
Ncat: Connected to 10.99.50.1:80.
Ncat: 0 bytes sent, 0 bytes received in 0.01 seconds.
```

### Basic virtual machine example with Multus

```yaml
---
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: fedora-vm
spec:
  running: false
  template:
    metadata:
      labels:
        kubevirt.io/vm: fedora-vm
      annotations:
        kubevirt.io/allow-pod-bridge-network-live-migration: "true"

    spec:
      evictionStrategy: LiveMigrate
      domain:
        cpu:
          cores: 2
        resources:
          requests:
            memory: 4G
        devices:
          disks:
            - name: fedora-vm-pvc
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
          - name: external
            bridge: {} # We use the bridge interface.
      networks:
        - name: external
          multus:
            networkName: namespace/networkattachmentdefinition # This is the NetworkAttachmentDefinition. See multus docs for more info.
      volumes:
        - name: fedora-vm-pvc
          persistentVolumeClaim:
            claimName: fedora-vm-pvc
        - name: cloudinitdisk
          cloudInitNoCloud:
            networkData: |
              network:
                version: 1
                config:
                  - type: physical
                    name: eth0
                    subnets:
                      - type: dhcp
            userData: |-
              #cloud-config
              users:
                - name: cloud-user
                  ssh_authorized_keys:
                    - ssh-rsa ....
                  sudo: ['ALL=(ALL) NOPASSWD:ALL']
                  groups: sudo
                  shell: /bin/bash
              runcmd:
                - "sudo touch /root/installed"
                - "sudo dnf update"
                - "sudo dnf install httpd fastfetch -y"
                - "sudo systemctl daemon-reload"
                - "sudo systemctl enable httpd"
                - "sudo systemctl start --no-block httpd"

  dataVolumeTemplates:
  - metadata:
      name: fedora-vm-pvc
    spec:
      storage:
        resources:
          requests:
            storage: 35Gi
        accessModes:
          - ReadWriteMany
        storageClassName: "nfs-csi"
      source:
        http:
          url: "https://fedora.mirror.wearetriple.com/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2"
```

In this example we will create a virtual machine that is bound to the bridge interface with the help of [Multus]({{< relref "../kubernetes-guides/network/multus" >}}).
You can start the machine with `kubectl virt start fedora-vm`.
After that you can look up the ip address of the virtual machine with

```bash
kubectl get vmi -owide

NAME        AGE    PHASE     IP            NODENAME   READY   LIVE-MIGRATABLE   PAUSED
fedora-vm   6d9h   Running   10.99.101.53   kube1      True    True
```

## Other forms of management

There is a project called [KubeVirt-Manager](https://kubevirt-manager.io/) for managing virtual machines with KubeVirt through a nice web interface.
You can also choose to deploy virtual machines with ArgoCD of Flux.

## Documentation

KubeVirt has a huge documentation page where you can check out everything on running virtual machines with KubeVirt.
The documentation can be found [here](https://kubevirt.io/user-guide/).
