---
title: "Disaster Recovery"
description: "Procedure for snapshotting etcd database and recovering from catastrophic control plane failure."
---

`etcd` database backs Kubernetes control plane state, so if the `etcd` service is unavailable
Kubernetes control plane goes down, and the cluster is not recoverable until `etcd` is recovered with contents.
The `etcd` consistency model builds around the consensus protocol Raft, so for highly-available control plane clusters,
loss of one control plane node doesn't impact cluster health.
In general, `etcd` stays up as long as a sufficient number of nodes to maintain quorum are up.
For a three control plane node Talos cluster, this means that the cluster tolerates a failure of any single node,
but losing more than one node at the same time leads to complete loss of service.
Because of that, it is important to take routine backups of `etcd` state to have a snapshot to recover cluster from
in case of catastrophic failure.

## Backup

### Snapshotting `etcd` Database

Create a consistent snapshot of `etcd` database with `talosctl etcd snapshot` command:

```bash
$ talosctl -n <IP> etcd snapshot db.snapshot
etcd snapshot saved to "db.snapshot" (2015264 bytes)
snapshot info: hash c25fd181, revision 4193, total keys 1287, total size 3035136
```

> Note: filename `db.snapshot` is arbitrary.

This database snapshot can be taken on any healthy control plane node (with IP address `<IP>` in the example above),
as all `etcd` instances contain exactly same data.
It is recommended to configure `etcd` snapshots to be created on some schedule to allow point-in-time recovery using the latest snapshot.

### Disaster Database Snapshot

If `etcd` cluster is not healthy, the `talosctl etcd snapshot` command might fail.
In that case, copy the database snapshot directly from the control plane node:

```bash
talosctl -n <IP> cp /var/lib/etcd/member/snap/db .
```

This snapshot might not be fully consistent (if the `etcd` process is running), but it allows
for disaster recovery when latest regular snapshot is not available.

### Machine Configuration

Machine configuration might be required to recover the node after hardware failure.
Backup Talos node machine configuration with the command:

```bash
talosctl -n IP get mc v1alpha1 -o yaml | yq eval '.spec' -
```

## Recovery

Before starting a disaster recovery procedure, make sure that `etcd` cluster can't be recovered:

* get `etcd` cluster member list on all healthy control plane nodes with `talosctl -n IP etcd members` command and compare across all members.
* query `etcd` health across control plane nodes with `talosctl -n IP service etcd`.

If the quorum can be restored, restoring quorum might be a better strategy than performing full disaster recovery
procedure.

### Latest Etcd Snapshot

Get hold of the latest `etcd` database snapshot.
If a snapshot is not fresh enough, create a database snapshot (see above),  even if the `etcd` cluster is unhealthy.

### Init Node

Make sure that there are no control plane nodes with machine type `init`:

```bash
$ talosctl -n <IP1>,<IP2>,... get machinetype
NODE         NAMESPACE   TYPE          ID             VERSION   TYPE
172.20.0.2   config      MachineType   machine-type   2         controlplane
172.20.0.4   config      MachineType   machine-type   2         controlplane
172.20.0.3   config      MachineType   machine-type   2         controlplane
```

Nodes with `init` type are incompatible with `etcd` recovery procedure.
`init` node can be converted to `controlplane` type with `talosctl edit mc --mode=staged` command followed
by node reboot with `talosctl reboot` command.

### Preparing Control Plane Nodes

If some control plane nodes experienced hardware failure, replace them with new nodes.
Use machine configuration backup to re-create the nodes with the same secret material and control plane settings
to allow workers to join the recovered control plane.

If a control plane node is healthy but `etcd` isn't, wipe the node's `EPHEMERAL` partition to remove the `etcd`
data directory (make sure a database snapshot is taken before doing this):

```bash
talosctl -n <IP> reset --graceful=false --reboot --system-labels-to-wipe=EPHEMERAL
```

At this point, all control plane nodes should boot up, and `etcd` service should be in the `Preparing` state.

Kubernetes control plane endpoint should be pointed to the new control plane nodes if there were
any changes to the node addresses.

### Recovering from the Backup

Make sure all `etcd` service instances are in `Preparing` state:

```bash
$ talosctl -n <IP> service etcd
NODE     172.20.0.2
ID       etcd
STATE    Preparing
HEALTH   ?
EVENTS   [Preparing]: Running pre state (17s ago)
         [Waiting]: Waiting for service "cri" to be "up", time sync (18s ago)
         [Waiting]: Waiting for service "cri" to be "up", service "networkd" to be "up", time sync (20s ago)
```

Execute the bootstrap command against any control plane node passing the path to the `etcd` database snapshot:

```bash
$ talosctl -n <IP> bootstrap --recover-from=./db.snapshot
recovering from snapshot "./db.snapshot": hash c25fd181, revision 4193, total keys 1287, total size 3035136
```

> Note: if database snapshot was copied out directly from the `etcd` data directory using `talosctl cp`,
> add flag `--recover-skip-hash-check` to skip integrity check on restore.

Talos node should print matching information in the kernel log:

```log
recovering etcd from snapshot: hash c25fd181, revision 4193, total keys 1287, total size 3035136
{"level":"info","msg":"restoring snapshot","path":"/var/lib/etcd.snapshot","wal-dir":"/var/lib/etcd/member/wal","data-dir":"/var/lib/etcd","snap-dir":"/var/li}
{"level":"info","msg":"restored last compact revision","meta-bucket-name":"meta","meta-bucket-name-key":"finishedCompactRev","restored-compact-revision":3360}
{"level":"info","msg":"added member","cluster-id":"a3390e43eb5274e2","local-member-id":"0","added-peer-id":"eb4f6f534361855e","added-peer-peer-urls":["https:/}
{"level":"info","msg":"restored snapshot","path":"/var/lib/etcd.snapshot","wal-dir":"/var/lib/etcd/member/wal","data-dir":"/var/lib/etcd","snap-dir":"/var/lib/etcd/member/snap"}
```

Now `etcd` service should become healthy on the bootstrap node, Kubernetes control plane components
should start and control plane endpoint should become available.
Remaining control plane nodes join `etcd` cluster once control plane endpoint is up.

## Single Control Plane Node Cluster

This guide applies to the single control plane clusters as well.
In fact, it is much more important to take regular snapshots of the `etcd` database in single control plane node
case, as loss of the control plane node might render the whole cluster irrecoverable without a backup.
