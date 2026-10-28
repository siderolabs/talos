---
description: |
    ContainerConfig is a container configuration document.
    ContainerConfig declares a container to be run by Talos directly, without Kubernetes.

    The container is started as soon as the configuration is applied, with no image rebuild
    and no reboot. It runs against the CRI containerd instance in the dedicated
    `taloscontainers` namespace, and is restarted automatically 5 seconds after it stops.

    Containers are not Talos services: they do not appear in `talosctl services`, and
    `talosctl service` does not apply to them. Status is reported via `ContainerStatus`.
title: ContainerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ContainerConfig
name: nginx # Name of the container.
image: docker.io/library/nginx:1.27 # OCI image reference supplying the container's root filesystem.
# Environment variables, in `KEY=value` form, merged over the image's own ENV.
environment:
    - NGINX_PORT=8080
# Filesystems to mount into the container.
mounts:
    - # Mount a user, existing or external volume by name.
      volume:
        name: web-content # Name of the volume to mount.
        destination: /usr/share/nginx/html # Absolute path inside the container's mount namespace.
        # Mount options. Volume mounts are writable by default (`rw`).
        options:
            - ro
    - # Mount a tmpfs for scratch space.
      tmpfs:
        destination: /tmp # Absolute path inside the container's mount namespace.
        size: 64MiB # Size of the tmpfs, e.g. `64MiB`. Empty means the kernel default.
# Resource limits, applied as cgroup v2 settings.
resources:
    # Hard ceilings the container cannot exceed.
    limits:
        cpu: 1500m # CPU ceiling in millicores, mapped onto cgroup v2 `cpu.max`.
        memory: 512MiB # Memory ceiling, mapped onto cgroup v2 `memory.max`.

        # # Swap ceiling, mapped onto cgroup v2 `memory.swap.max`.
        # swap: "0"
# Conditions which must be satisfied before the container is started.
dependsOn:
    # Host paths which must exist before the container starts.
    paths:
        - /var/mnt/web-content
    # Network readiness conditions which must be satisfied.
    network:
        - addresses
    time: true # Whether the clock must be synchronized before the container starts.

# # Devices to hand to the container.
# devices:
#     - # Hand a raw volume to the container as a block device.
#       rawVolume:
#         name: data # Name of the `RawVolumeConfig` document.
#         destination: /dev/data # Path of the device node inside the container.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the container.<br><br>Must be between 1 and 63 characters long, and can only contain lowercase ASCII<br>letters, digits and hyphens. It is used as the containerd container ID and as the<br>container's log identifier, so it may not collide with a Talos service name.  | |
|`image` |string |OCI image reference supplying the container's root filesystem.<br><br>A digest-pinned reference (`repo@sha256:...`) is recommended: it is the only form<br>that guarantees the same bytes on every pull. Short references are accepted and<br>normalized, so `nginx` becomes `index.docker.io/library/nginx:latest`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: docker.io/library/nginx:1.27
{{< /highlight >}}</details> | |
|`entrypoint` |[]string |Overrides the image's ENTRYPOINT.<br><br>Unset means the image's own entrypoint is used.  | |
|`args` |[]string |Overrides the image's CMD.  | |
|`workingDir` |string |Overrides the image's WORKDIR.  | |
|`runAs` |<a href="#ContainerConfig.runAs">ContainerRunAs</a> |Overrides the image's USER uid and/or gid.<br><br>There are no user namespaces, so a container running as uid 0 is root on the host.  | |
|`environment` |[]string |Environment variables, in `KEY=value` form, merged over the image's own ENV.<br><br>Values are stored in the machine configuration verbatim, so treat anything put here<br>as being as sensitive as the machine configuration itself.  | |
|`mounts` |<a href="#ContainerConfig.mounts.">[]ContainerMount</a> |Filesystems to mount into the container.  | |
|`devices` |<a href="#ContainerConfig.devices.">[]ContainerDevice</a> |Devices to hand to the container. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
devices:
    - # Hand a raw volume to the container as a block device.
      rawVolume:
        name: data # Name of the `RawVolumeConfig` document.
        destination: /dev/data # Path of the device node inside the container.
{{< /highlight >}}</details> | |
|`security` |<a href="#ContainerConfig.security">ContainerSecurity</a> |Security settings for the container.  | |
|`network` |<a href="#ContainerConfig.network">ContainerNetwork</a> |Network settings for the container.  | |
|`resources` |<a href="#ContainerConfig.resources">ContainerResources</a> |Resource limits, applied as cgroup v2 settings.  | |
|`dependsOn` |<a href="#ContainerConfig.dependsOn">ContainerDependsOn</a> |Conditions which must be satisfied before the container is started.  | |




## runAs {#ContainerConfig.runAs}

ContainerRunAs overrides the image's user and group.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`uid` |int32 |UID to run the container's entrypoint as.<br><br>Unset means use the image's own USER. There are no user namespaces, so uid 0 is host<br>root.<br><br>Required when the image's USER names its user rather than giving a numeric uid<br>(e.g. `app` or `app:app`): Talos does not read the image's /etc/passwd, and rejects<br>such a container instead of running it as root. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
uid: 65534
{{< /highlight >}}</details> | |
|`gid` |int32 |GID to run the container's entrypoint as.<br><br>Unset means use the image's own USER.<br><br>Required when the image's USER names its group (e.g. `app:app`), or names only a user<br>(e.g. `app`), whose primary group would come from the image's /etc/passwd: Talos does<br>not read it, and rejects such a container instead of running it as root. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
gid: 65534
{{< /highlight >}}</details> | |






## mounts[] {#ContainerConfig.mounts.}

ContainerMount describes a single filesystem to mount into the container.

Exactly one source must be set. Raw OCI mounts are deliberately not exposed; every source is
typed so that Talos can reason about what a container is allowed to reach.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volume` |<a href="#ContainerConfig.mounts..volume">VolumeMount</a> |Mount a user, existing or external volume by name.<br><br>The name is that of a `UserVolumeConfig`, `ExistingVolumeConfig` or<br>`ExternalVolumeConfig` document. Volume names are unique across those kinds, so the<br>name resolves to one volume. The volume must be declared in the same configuration.<br><br>The volume is mounted from `/var/mnt/<name>` on the host. Declaring this mount also<br>makes the container wait for the volume to be mounted before it starts.  | |
|`tmpfs` |<a href="#ContainerConfig.mounts..tmpfs">TmpfsMount</a> |Mount a tmpfs for scratch space.  | |
|`hostPath` |<a href="#ContainerConfig.mounts..hostPath">HostPathMount</a> |Bind-mount a path from the host.<br><br>The source must already exist; Talos will not create it. This is the widest of the<br>three sources and the only one that can reach arbitrary host state.  | |




### volume {#ContainerConfig.mounts..volume}

VolumeMount mounts a volume by name.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the volume to mount.<br><br>The name of a `UserVolumeConfig`, `ExistingVolumeConfig` or `ExternalVolumeConfig` document.  | |
|`destination` |string |Absolute path inside the container's mount namespace.  | |
|`options` |[]string |Mount options. Volume mounts are writable by default (`rw`).<br><br>A volume whose `ExistingVolumeConfig` or `ExternalVolumeConfig` declares it<br>read-only must be mounted with `ro`.  |`ro`<br />`rw`<br />`noexec`<br />`nosuid`<br />`nodev`<br />`noatime`<br />`rbind`<br />`rshared`<br /> |






### tmpfs {#ContainerConfig.mounts..tmpfs}

TmpfsMount mounts a tmpfs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`destination` |string |Absolute path inside the container's mount namespace.  | |
|`size` |string |Size of the tmpfs, e.g. `64MiB`. Empty means the kernel default. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
size: 64MiB
{{< /highlight >}}</details> | |
|`options` |[]string |Mount options. Tmpfs mounts are writable by default (`rw`).  | |






### hostPath {#ContainerConfig.mounts..hostPath}

HostPathMount bind-mounts a host path.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`source` |string |Absolute path on the host. Must already exist.  | |
|`destination` |string |Absolute path inside the container's mount namespace.  | |
|`options` |[]string |Mount options. Host path mounts are writable by default (`rw`).  | |








## devices[] {#ContainerConfig.devices.}

ContainerDevice describes a single device to hand to the container.

Exactly one source must be set.




{{< highlight yaml >}}
devices:
    - # Hand a raw volume to the container as a block device.
      rawVolume:
        name: data # Name of the `RawVolumeConfig` document.
        destination: /dev/data # Path of the device node inside the container.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rawVolume` |<a href="#ContainerConfig.devices..rawVolume">RawVolumeDevice</a> |Hand a raw volume to the container as a block device.<br><br>The raw volume is referenced by the name of its `RawVolumeConfig` document.<br>The container gets a device node at the destination and is allowed to read and write<br>that device, and no other block device. This works with the `restricted` profile,<br>no capabilities are added. For an encrypted raw volume the node is the unlocked device.<br><br>Declaring this device makes the container wait for the raw volume to be ready, and the<br>raw volume is not closed while the container is running. A raw volume can be handed to<br>a single container only.  | |




### rawVolume {#ContainerConfig.devices..rawVolume}

RawVolumeDevice hands a raw volume to the container by name.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the `RawVolumeConfig` document.  | |
|`destination` |string |Path of the device node inside the container.<br><br>Must be under `/dev/`, and not one of the entries the container runtime creates there<br>itself (e.g. `/dev/null`, `/dev/pts`). The node keeps the owner and mode of the device<br>on the host. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
destination: /dev/data
{{< /highlight >}}</details> | |








## security {#ContainerConfig.security}

ContainerSecurity configures the container's security posture.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`profile` |ContainerSecurityProfile |Security profile.<br><br>`restricted` drops all capabilities, allows no device access, and mounts the rootfs<br>and sysfs read-only. `privileged` grants all grantable capabilities and all devices,<br>and mounts sysfs read-write.  |`restricted`<br />`privileged`<br /> |
|`capabilities` |<a href="#ContainerConfig.security.capabilities">ContainerCapabilities</a> |Linux capabilities to add or drop on top of the profile.  | |
|`machinedAccess` |bool |Publishes the container's PID so machined's API can recognize it, and bind-mounts the<br>machined API socket into the container.<br><br>This alone does not grant DAC access to the socket, which is owned by the `apid` user:<br>reaching it in practice still requires `profile: privileged` or an equivalent capability/<br>`runAs` grant. Once connected, the container may request any role, same as extension<br>services; the RPC's own role requirements are what actually gate access.  | |




### capabilities {#ContainerConfig.security.capabilities}

ContainerCapabilities adjusts the container's Linux capabilities.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`add` |[]string |Capabilities to grant, without the `CAP_` prefix. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
add:
    - NET_ADMIN
{{< /highlight >}}</details> | |
|`drop` |[]string |Capabilities to remove, without the `CAP_` prefix. `ALL` removes every capability. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
drop:
    - ALL
{{< /highlight >}}</details> | |








## network {#ContainerConfig.network}

ContainerNetwork configures the container's network namespace.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`mode` |ContainerNetworkMode |Network mode.<br><br>`none` gives the container its own empty network namespace with no host access.<br>`host` shares the host network namespace, so the container sees every interface and<br>can bind any port.  |`none`<br />`host`<br /> |






## resources {#ContainerConfig.resources}

ContainerResources configures cgroup v2 resource limits.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`limits` |<a href="#ContainerConfig.resources.limits">ContainerResourceLimits</a> |Hard ceilings the container cannot exceed.  | |




### limits {#ContainerConfig.resources.limits}

ContainerResourceLimits are hard ceilings.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpu` |string |CPU ceiling in millicores, mapped onto cgroup v2 `cpu.max`.<br><br>`1000m` is one core. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpu: 1500m
{{< /highlight >}}</details> | |
|`memory` |string |Memory ceiling, mapped onto cgroup v2 `memory.max`.<br><br>Exceeding it OOM-kills the container. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
memory: 512MiB
{{< /highlight >}}</details> | |
|`swap` |string |Swap ceiling, mapped onto cgroup v2 `memory.swap.max`.<br><br>`0` keeps the container's memory out of swap entirely, so its memory ceiling is enforced<br>by the OOM killer rather than by swapping. Unset leaves the container free to use the<br>node's swap. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
swap: "0"
{{< /highlight >}}</details> | |








## dependsOn {#ContainerConfig.dependsOn}

ContainerDependsOn gates container startup on external conditions.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`paths` |[]string |Host paths which must exist before the container starts.<br><br>Polled, so a path that never appears leaves the container waiting indefinitely. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
paths:
    - /var/mnt/web-content
{{< /highlight >}}</details> | |
|`network` |[]string |Network readiness conditions which must be satisfied.  |`addresses`<br />`connectivity`<br />`hostname`<br />`etcfiles`<br /> |
|`time` |bool |Whether the clock must be synchronized before the container starts.  | |
|`containers` |[]string |Other containers, by document name, which must be running first.<br><br>Cycles are rejected when the machine configuration is applied.  | |








