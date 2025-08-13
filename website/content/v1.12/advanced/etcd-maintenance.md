---
title: "etcd Maintenance"
description: "Operational instructions for etcd database."
---

`etcd` database backs Kubernetes control plane state, so `etcd` health is critical for Kubernetes availability.

> Note: Commands from `talosctl etcd` namespace are functional only on the Talos control plane nodes.
> Each time you see `<IPx>` in this page, it is referencing IP address of control plane node.

## Space Quota

`etcd` default database space quota is set to 2 GiB by default.
If the database size exceeds the quota, `etcd` will stop operations until the issue is resolved.

This condition can be checked with `talosctl etcd alarm list` command:

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP> etcd alarm list
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             ALARM
172.20.0.2   a49c021e76e707db   NOSPACE
{{< /tab >}}
{{< /tabpane >}}

If the Kubernetes database contains lots of resources, space quota can be increased to match the actual usage.
The recommended maximum size is 8 GiB.

To increase the space quota, edit the `etcd` section in the machine configuration:

```yaml
cluster:
  etcd:
    extraArgs:
      quota-backend-bytes: 4294967296 # 4 GiB
```

Once the node is rebooted with the new configuration, use `talosctl etcd alarm disarm` to clear the `NOSPACE` alarm.

## Defragmentation

`etcd` database can become fragmented over time if there are lots of writes and deletes.
Kubernetes API server performs automatic compaction of the `etcd` database, which marks deleted space as free and ready to be reused.
However, the space is not actually freed until the database is defragmented.

If the database is heavily fragmented (in use/db size ratio is less than 0.5), defragmentation might increase the performance.
If the database runs over the space quota (see above), but the actual in use database size is small, defragmentation is required to bring the on-disk database size below the limit.

Current database size can be checked with `talosctl etcd status` command:

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1>,<IP2>,<IP3> etcd status
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             DB SIZE   IN USE            LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   ERRORS
172.20.0.3   ecebb05b59a776f1   21 MB     6.0 MB (29.08%)   ecebb05b59a776f1   53391        4           53391                false
172.20.0.2   a49c021e76e707db   17 MB     4.5 MB (26.10%)   ecebb05b59a776f1   53391        4           53391                false
172.20.0.4   eb47fb33e59bf0e2   20 MB     5.9 MB (28.96%)   ecebb05b59a776f1   53391        4           53391                false
{{< /tab >}}
{{< /tabpane >}}

If any of the nodes are over database size quota, alarms will be printed in the `ERRORS` column.

To defragment the database, run `talosctl etcd defrag` command:

```bash
talosctl -n <IP1> etcd defrag
```

> Note: Defragmentation is a resource-intensive operation, so it is recommended to run it on a single node at a time.
> Defragmentation to a live member blocks the system from reading and writing data while rebuilding its state.

Once the defragmentation is complete, the database size will match closely to the in use size:

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1> etcd status
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             DB SIZE   IN USE             LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   ERRORS
172.20.0.2   a49c021e76e707db   4.5 MB    4.5 MB (100.00%)   ecebb05b59a776f1   56065        4           56065                false
{{< /tab >}}
{{< /tabpane >}}

## Snapshotting

Regular backups of `etcd` database should be performed to ensure that the cluster can be restored in case of a failure.
This procedure is described in the [disaster recovery]({{< relref "disaster-recovery" >}}) guide.

## Downgrade v3.6 to v3.5

Before beginning, check the `etcd` health and download snapshot, as described in [disaster recovery]({{< relref "disaster-recovery" >}}).
Should something go wrong with the downgrade, it is possible to use this backup to rollback to existing `etcd` version.

This example shows how to downgrade an `etcd` in Talos cluster.

### Step 1: Check Downgrade Requirements

Is the cluster healthy and running v3.6.x?

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1>,<IP2>,<IP3> etcd status
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             DB SIZE   IN USE            LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   PROTOCOL   STORAGE   ERRORS
172.20.0.4   a2b8a1f794bdd561   3.6 MB    2.2 MB (61.59%)   a49c021e76e707db   4703         2           4703                 false     3.6.4      3.6.0
172.20.0.3   912415ee6ed360c4   3.5 MB    2.2 MB (61.88%)   a49c021e76e707db   4703         2           4703                 false     3.6.4      3.6.0
172.20.0.2   a49c021e76e707db   3.5 MB    2.2 MB (62.06%)   a49c021e76e707db   4703         2           4703                 false     3.6.4      3.6.0
{{< /tab >}}
{{< /tabpane >}}

### Step 2: Download Snapshot

[Download the snapshot backup]({{< relref "disaster-recovery" >}}) to provide a downgrade path should any problems occur.

### Step 3: Validate Downgrade

Validate the downgrade target version before enabling the downgrade:

- We only support downgrading one minor version at a time, e.g. downgrading from v3.6 to v3.4 isn't allowed.
- Please do not move on to next step until the validation is successful.

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1> etcd downgrade validate 3.5
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MESSAGE
172.20.0.2   downgrade validate success, cluster version 3.6
{{< /tab >}}
{{< /tabpane >}}

### Step 4: Enable Downgrade

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1> etcd downgrade enable 3.5
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MESSAGE
172.20.0.2   downgrade enable success, cluster version 3.6
{{< /tab >}}
{{< /tabpane >}}

After enabling downgrade, the cluster will start to operate with v3.5 protocol, which is the downgrade target version.
In addition, `etcd` will automatically migrate the schema to the downgrade target version, which usually happens very fast.
Confirm the storage version of all servers has been migrated to v3.5 by checking the endpoint status before moving on to the next step.

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1>,<IP2>,<IP3> etcd status
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             DB SIZE   IN USE            LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   PROTOCOL   STORAGE   ERRORS
172.20.0.3   912415ee6ed360c4   3.5 MB    1.9 MB (54.92%)   a49c021e76e707db   5152         2           5152                 false     3.6.4      3.5.0
172.20.0.2   a49c021e76e707db   3.5 MB    1.9 MB (54.64%)   a49c021e76e707db   5152         2           5152                 false     3.6.4      3.5.0
172.20.0.4   a2b8a1f794bdd561   3.6 MB    1.9 MB (54.44%)   a49c021e76e707db   5152         2           5152                 false     3.6.4      3.5.0
{{< /tab >}}
{{< /tabpane >}}

> Note: Once downgrade is enabled, the cluster will remain operating with v3.5 protocol even if all the servers are still running the v3.6 binary, unless the downgrade is canceled with `talosctl -n <IP1> downgrade cancel`.

### Step 5: Patch Machine Config

Before patching the node, check if the etcd is leader.
We recommend downgrading the leader last.
If the server to be downgraded is the leader, you can avoid some downtime by `forfeit-leadership` to another server before stopping this server.

```bash
talosctl -n <IP1> etcd forfeit-leadership
```

Create a file with the patch pointing to desired `etcd` image:

```yaml
# etcd-patch.yaml
cluster:
  etcd:
    image: gcr.io/etcd-development/etcd:v3.5.22
```

Apply patch to the machine with same configuration but with the new `etcd` version.

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1> patch machineconfig --patch @etcd-patch.yaml --mode reboot
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
patched MachineConfigs.config.talos.dev/v1alpha1 at the node 172.20.0.2
Applied configuration with a reboot
{{< /tab >}}
{{< /tabpane >}}

Verify that each member, and then the entire cluster, becomes healthy with the new v3.5 `etcd`:

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1>,<IP2>,<IP3> etcd status
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             DB SIZE   IN USE            LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   PROTOCOL   STORAGE   ERRORS
172.20.0.2   a49c021e76e707db   3.5 MB    3.1 MB (88.05%)   a2b8a1f794bdd561   13116        4           13116                false     3.5.22     3.5.0
172.20.0.4   a2b8a1f794bdd561   3.6 MB    3.1 MB (88.12%)   a2b8a1f794bdd561   13116        4           13116                false     3.6.4      3.5.0
172.20.0.3   912415ee6ed360c4   3.5 MB    3.1 MB (88.30%)   a2b8a1f794bdd561   13116        4           13116                false     3.6.4      3.5.0
{{< /tab >}}
{{< /tabpane >}}

### Step 6: Continue on the Remaining Control Plane Nodes

When all members are downgraded, check the health and status of the cluster, and confirm the minor version of all members is v3.5, and storage version is empty:

{{< tabpane >}}
{{< tab header="Command" lang="Bash" >}}
talosctl -n <IP1>,<IP2>,<IP3> etcd status
{{< /tab >}}
{{< tab header="Output" lang="Console" >}}
NODE         MEMBER             DB SIZE   IN USE             LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   PROTOCOL   STORAGE   ERRORS
172.20.0.2   a49c021e76e707db   4.5 MB    4.5 MB (100.00%)   912415ee6ed360c4   13865        5           13865                false     3.5.22     3.5.0
172.20.0.4   a2b8a1f794bdd561   4.6 MB    4.6 MB (100.00%)   912415ee6ed360c4   13865        5           13865                false     3.5.22     3.5.0
172.20.0.3   912415ee6ed360c4   4.6 MB    4.6 MB (99.64%)    912415ee6ed360c4   13865        5           13865                false     3.5.22     3.5.0
{{< /tab >}}
{{< /tabpane >}}
