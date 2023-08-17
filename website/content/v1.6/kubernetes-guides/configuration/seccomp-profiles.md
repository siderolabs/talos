---
title: "Seccomp Profiles"
description: "Using custom Seccomp Profiles with Kubernetes workloads."
aliases:
  - ../../guides/pod-security
---

Seccomp stands for secure computing mode and has been a feature of the Linux kernel since version 2.6.12.
It can be used to sandbox the privileges of a process, restricting the calls it is able to make from userspace into the kernel.

Refer the [Kubernetes Seccomp Guide](https://kubernetes.io/docs/tutorials/security/seccomp/) for more details.

In this guide we are going to configure a custom Seccomp Profile that logs all syscalls made by the workload.

## Preparing the nodes

Create a machine config path with the contents below and save as `patch.yaml`

```yaml
machine:
  seccompProfiles:
    - name: audit.json
      value:
        defaultAction: SCMP_ACT_LOG
```

Apply the machine config to all the nodes using talosctl:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> patch mc -p @patch.yaml
```

This would create a seccomp profile name `audit.json` on the node at `/var/lib/kubelet/seccomp/profiles`.

The profiles can be used by Kubernetes pods by specfying the pod `securityContext` as below:

```yaml
spec:
  securityContext:
    seccompProfile:
      type: Localhost
      localhostProfile: profiles/audit.json
```

> Note that the `localhostProfile` uses the name of the profile created under `profiles` directory.
> So make sure to use path as `profiles/<profile-name.json>`

This can be verfied by running the below commands:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> get seccompprofiles
```

An output similar to below can be observed:

```text
NODE       NAMESPACE   TYPE             ID           VERSION
10.5.0.3   cri         SeccompProfile   audit.json   1
```

The content of the seccomp profile can be viewed by running the below command:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> read /var/lib/kubelet/seccomp/profiles/audit.json
```

An output similar to below can be observed:

```text
{"defaultAction":"SCMP_ACT_LOG"}
```

## Create a Kubernetes workload that uses the custom Seccomp Profile

Here we'll be using an example workload from the Kubernetes [documentation](https://kubernetes.io/docs/tutorials/security/seccomp/).

First open up a second terminal and run the following talosctl command so that we can view the Syscalls being logged in realtime:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> dmesg --follow --tail
```

Now deploy the example workload from the Kubernetes documentation:

```bash
kubectl apply -f https://k8s.io/examples/pods/security/seccomp/ga/audit-pod.yaml
```

Once the pod starts running the terminal running `talosctl dmesg` command from above should log similar to below:

```text
10.5.0.3: kern:    info: [2022-07-28T11:49:42.489473063Z]: cni0: port 1(veth32488a86) entered blocking state
10.5.0.3: kern:    info: [2022-07-28T11:49:42.490852063Z]: cni0: port 1(veth32488a86) entered disabled state
10.5.0.3: kern:    info: [2022-07-28T11:49:42.492470063Z]: device veth32488a86 entered promiscuous mode
10.5.0.3: kern:    info: [2022-07-28T11:49:42.503105063Z]: IPv6: ADDRCONF(NETDEV_CHANGE): eth0: link becomes ready
10.5.0.3: kern:    info: [2022-07-28T11:49:42.503944063Z]: IPv6: ADDRCONF(NETDEV_CHANGE): veth32488a86: link becomes ready
10.5.0.3: kern:    info: [2022-07-28T11:49:42.504764063Z]: cni0: port 1(veth32488a86) entered blocking state
10.5.0.3: kern:    info: [2022-07-28T11:49:42.505423063Z]: cni0: port 1(veth32488a86) entered forwarding state
10.5.0.3: kern: warning: [2022-07-28T11:49:44.873616063Z]: kauditd_printk_skb: 14 callbacks suppressed
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.873619063Z]: audit: type=1326 audit(1659008985.445:25): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=3 compat=0 ip=0x55ec0657bd3b code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.876609063Z]: audit: type=1326 audit(1659008985.445:26): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=3 compat=0 ip=0x55ec0657bd3b code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.878789063Z]: audit: type=1326 audit(1659008985.449:27): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=257 compat=0 ip=0x55ec0657bdaa code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.886693063Z]: audit: type=1326 audit(1659008985.461:28): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=202 compat=0 ip=0x55ec06532b43 code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.888764063Z]: audit: type=1326 audit(1659008985.461:29): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=202 compat=0 ip=0x55ec06532b43 code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.891009063Z]: audit: type=1326 audit(1659008985.461:30): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=1 compat=0 ip=0x55ec0657bd3b code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.893162063Z]: audit: type=1326 audit(1659008985.461:31): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=3 compat=0 ip=0x55ec0657bd3b code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.895365063Z]: audit: type=1326 audit(1659008985.461:32): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=39 compat=0 ip=0x55ec066eb68b code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.898306063Z]: audit: type=1326 audit(1659008985.461:33): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="runc:[2:INIT]" exe="/" sig=0 arch=c000003e syscall=59 compat=0 ip=0x55ec0657be16 code=0x7ffc0000
10.5.0.3: kern:  notice: [2022-07-28T11:49:44.901518063Z]: audit: type=1326 audit(1659008985.473:34): auid=4294967295 uid=0 gid=0 ses=4294967295 pid=2784 comm="http-echo" exe="/http-echo" sig=0 arch=c000003e syscall=158 compat=0 ip=0x455f35 code=0x7ffc0000
```

## Cleanup

You can clean up the test resources by running the following command:

```bash
kubectl delete pod audit-pod
```
