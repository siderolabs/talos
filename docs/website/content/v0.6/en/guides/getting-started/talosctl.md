---
title: The talosctl
---

One of the important components of Talos is the CLI (Command Line Interface) which let's you interact with the OS running on your system.
This guide gives you some hands on examples, and some more context when working with `talosctl`.

### Getting Started

To get going with `talosctl` you need to download the latest release from Github [here](https://github.com/talos-systems/talos/releases).

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

Now, test if it's working by running:

```bash
talosctl --help
```

### Commands

#### Configuration of talosctl

The `talosctl` command needs some configuration options to connect to the right node.
By default `talosctl` looks for a configuration file called `config` located at `$HOME/.talos`.

If you created the configuration file using one of the guides, you'll have a file named: `talosconfig` which you can place inside the `.talos` directory and name it `config` for `talosctl` to automatically use that specified configuration.

You can always override which configuration `talosctl` uses by specifing the `--talosconfig` parameter:

```bash
talosctl --talosconfig other_talosconfig
```

#### Connecting to a Node

> You need a working `talosconfig` before you can connect to a control-plane node, to get a `talosconfig` please follow one of the guides specific to your needs.
> We're assuming you have setup `talosctl` with a default `~/.talos/config`.

Connect to a controlplane:

```bash
talosctl config endpoint <IP/DNS name of controlplane node>
```

You can then switch to another node:

```bash
talosctl config nodes <IP/DNS name of node>
```

> Pro tip!
> You can connect to multiple nodes at once, by seperating it with a space like this:
>
> ```bash
> talosctl config nodes node1.exmaple.org node2.example.org
> ```
>
> You can use hostnames or ip's here as well, or mix and match.

To verify what node you're currently connected to, you can run:

```bash
$ talosctl version
Client:
        Tag:         v0.4.1
        SHA:         a1234bc5
        Built:
        Go version:  go1.14.2
        OS/Arch:     linux/amd64

Server:
        NODE:        192.168.2.44
        Tag:         v0.4.1
        SHA:         a1234bc5
        Built:
        Go version:  go1.14.2
        OS/Arch:     linux/amd64
```

This will output something like above.

### Getting Information From a Node

#### Services

Making sure all the services are running on a node is crucial for the operation of your Kubernetes cluster.
To identify all running services on a Talos node, you can run the `services` command.

```bash
$ talosctl services
NODE          SERVICE             STATE      HEALTH   LAST CHANGE     LAST EVENT
192.168.2.44   apid                Running    OK       192h7m40s ago   Health check successful
192.168.2.44   bootkube            Finished   ?        192h5m1s ago    Service finished successfully
192.168.2.44   containerd          Running    OK       192h7m47s ago   Health check successful
192.168.2.44   etcd                Running    OK       192h6m56s ago   Health check successful
192.168.2.44   kubelet             Running    OK       192h5m47s ago   Health check successful
192.168.2.44   machined-api        Running    ?        192h7m48s ago   Service started as goroutine
192.168.2.44   networkd            Running    OK       192h7m11s ago   Health check successful
192.168.2.44   ntpd                Running    ?        192h7m10s ago   Started task ntpd (PID 4144) for container ntpd
192.168.2.44   routerd             Running    OK       192h7m46s ago   Started task routerd (PID 3907) for container routerd
192.168.2.44   system-containerd   Running    OK       192h7m48s ago   Health check successful
192.168.2.44   trustd              Running    OK       192h7m45s ago   Health check successful
192.168.2.44   udevd               Running    ?        192h7m47s ago   Process Process(["/sbin/udevd" "--resolve-names=never" "-D"]) started with PID 2893
192.168.2.44   udevd-trigger       Finished   ?        192h7m47s ago   Service finished successfully
```

> Note: above command is run on a controlplane node, a worker node has different services.

#### Containers

Sometimes it's neccessary to check for certain containers on Talos itself.
This can be achieved by the `containers` subcommand:

```bash
$ talosctl containers
NODE          NAMESPACE   ID         IMAGE            PID    STATUS
192.168.2.44   system      apid       talos/apid       4021   RUNNING
192.168.2.44   system      networkd   talos/networkd   3893   RUNNING
192.168.2.44   system      ntpd       talos/ntpd       4144   RUNNING
192.168.2.44   system      routerd    talos/routerd    3907   RUNNING
192.168.2.44   system      trustd     talos/trustd     4010   RUNNING
```

> For the keyboard warriors: `talosctl c` works as well, saves you 9 characters.

To verify the contrainers running on the hosts that live in the Kubernetes namespace:

```bash
$ talosctl containers -k
NODE          NAMESPACE   ID                                                                         IMAGE                                                                                                         PID     STATUS
192.168.2.44   k8s.io      kube-system/coredns-669d45d65b-st7sl                                       k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      6632    RUNNING
192.168.2.44   k8s.io      └─ kube-system/coredns-669d45d65b-st7sl:coredns                            k8s.gcr.io/coredns@sha256:7ec975f167d815311a7136c32e70735f0d00b73781365df1befd46ed35bd4fe7                    6719    RUNNING
192.168.2.44   k8s.io      kube-system/coredns-669d45d65b-zt586                                       k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      6587    RUNNING
192.168.2.44   k8s.io      └─ kube-system/coredns-669d45d65b-zt586:coredns                            k8s.gcr.io/coredns@sha256:7ec975f167d815311a7136c32e70735f0d00b73781365df1befd46ed35bd4fe7                    6712    RUNNING
192.168.2.44   k8s.io      kube-system/kube-apiserver-6lrdp                                           k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      5511    RUNNING
192.168.2.44   k8s.io      └─ kube-system/kube-apiserver-6lrdp:kube-apiserver                         k8s.gcr.io/hyperkube:v1.18.0                                                                                  6167    RUNNING
192.168.2.44   k8s.io      kube-system/kube-controller-manager-p6zpr                                  k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      5807    RUNNING
192.168.2.44   k8s.io      └─ kube-system/kube-controller-manager-p6zpr:kube-controller-manager       k8s.gcr.io/hyperkube:v1.18.0                                                                                  5844    RUNNING
192.168.2.44   k8s.io      kube-system/kube-flannel-xr89l                                             k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      5152    RUNNING
192.168.2.44   k8s.io      └─ kube-system/kube-flannel-xr89l:install-cni                              quay.io/coreos/flannel-cni:v0.3.0                                                                             5332    RUNNING
192.168.2.44   k8s.io      └─ kube-system/kube-flannel-xr89l:kube-flannel                             quay.io/coreos/flannel:v0.11.0-amd64                                                                          5197    RUNNING
192.168.2.44   k8s.io      kube-system/kube-proxy-9bh74                                               k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      4999    RUNNING
192.168.2.44   k8s.io      └─ kube-system/kube-proxy-9bh74:kube-proxy                                 k8s.gcr.io/hyperkube:v1.18.0                                                                                  5031    RUNNING
192.168.2.44   k8s.io      kube-system/kube-scheduler-k87t8                                           k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      5714    RUNNING
192.168.2.44   k8s.io      └─ kube-system/kube-scheduler-k87t8:kube-scheduler                         k8s.gcr.io/hyperkube:v1.18.0                                                                                  5745    RUNNING
192.168.2.44   k8s.io      kube-system/pod-checkpointer-c5hk6                                         k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      5512    RUNNING
192.168.2.44   k8s.io      kube-system/pod-checkpointer-c5hk6-talos-10-32-2-197                       k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      6341    RUNNING
192.168.2.44   k8s.io      └─ kube-system/pod-checkpointer-c5hk6-talos-10-32-2-197:pod-checkpointer   docker.io/autonomy/pod-checkpointer@sha256:476277082931570df3c863ad37ab11f0ad7050710caf02ba46d053837fe6e366   6374    RUNNING
192.168.2.44   k8s.io      └─ kube-system/pod-checkpointer-c5hk6:pod-checkpointer                     docker.io/autonomy/pod-checkpointer@sha256:476277082931570df3c863ad37ab11f0ad7050710caf02ba46d053837fe6e366   5927    RUNNING
192.168.2.44   k8s.io      kubelet                                                                    k8s.gcr.io/hyperkube:v1.18.0                                                                                  4885    RUNNING
192.168.2.44   k8s.io      metallb-system/speaker-2rbf7                                               k8s.gcr.io/pause@sha256:f78411e19d84a252e53bff71a4407a5686c46983a2c2eeed83929b888179acea                      84985   RUNNING
192.168.2.44   k8s.io      └─ metallb-system/speaker-2rbf7:speaker                                    docker.io/metallb/speaker@sha256:2b74eca0f25e946e9a1dc4b94b9da067b1fec4244364d266283dfbbab546a629             85033   RUNNING
```

#### Logs

Retrieving logs is also done through `talosctl`.
Using the previous commands to look up containers, we can use the _ID_ to get the logs from a specific container.

```bash
$ talosctl logs apid
192.168.2.44: 2020/05/19 14:14:24.715975 provider.go:109: next renewal in 11h59m59.642046025s
192.168.2.44: 2020/05/19 14:14:34.684449 log.go:98: OK [/machine.MachineService/ServiceList] 5.355187ms stream Success (:authority=192.168.2.44:50000;content-type=application/grpc;user-agent=grpc-go/1.26.0)
192.168.2.44: 2020/05/19 14:16:04.379499 log.go:98: OK [/machine.MachineService/ServiceList] 2.60977ms stream Success (:authority=192.168.2.44:50000;content-type=application/grpc;user-agent=grpc-go/1.26.0)
192.168.2.44: 2020/05/19 14:17:50.498066 log.go:98: OK [/machine.MachineService/ServiceList] 2.489054ms stream Success (:authority=192.168.2.44:50000;content-type=application/grpc;user-agent=grpc-go/1.26.0)
.....
```

> To get kubernetes logs, you need to specify the `-k` parameter and the _ID_

#### Copy Files

Sometimes you just need to copy over some files from the host machine, and troubleshoot on you local machine.
This can be done through the `copy` command.

```bash
talosctl copy /var/log/pods/ ./pods
```

> You can also use `cp` instead of `copy`

This will copy all logs located in `/var/log/pods/` to your local machine in the directory `pods`.

### Next Steps

To get all options available, please have a look at the [Git repo](https://github.com/talos-systems/talos/blob/master/docs/talosctl/talosctl.md)
