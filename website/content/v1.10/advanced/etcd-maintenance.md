---
title: "etcd Maintenance"
description: "Operational instructions for etcd database."
---

`etcd` database backs Kubernetes control plane state, so `etcd` health is critical for Kubernetes availability.

## Space Quota

`etcd` default database space quota is set to 2 GiB by default.
If the database size exceeds the quota, `etcd` will stop operations until the issue is resolved.

This condition can be checked with `talosctl etcd alarm list` command:

```bash
$ talosctl -n <IP> etcd alarm list
NODE         MEMBER             ALARM
172.20.0.2   a49c021e76e707db   NOSPACE
```

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

```bash
$ talosctl -n <CP1>,<CP2>,<CP3> etcd status
NODE         MEMBER             DB SIZE   IN USE            LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   ERRORS
172.20.0.3   ecebb05b59a776f1   21 MB     6.0 MB (29.08%)   ecebb05b59a776f1   53391        4           53391                false
172.20.0.2   a49c021e76e707db   17 MB     4.5 MB (26.10%)   ecebb05b59a776f1   53391        4           53391                false
172.20.0.4   eb47fb33e59bf0e2   20 MB     5.9 MB (28.96%)   ecebb05b59a776f1   53391        4           53391                false
```

If any of the nodes are over database size quota, alarms will be printed in the `ERRORS` column.

To defragment the database, run `talosctl etcd defrag` command:

```bash
talosctl -n <CP1> etcd defrag
```

> Note: defragmentation is a resource-intensive operation, so it is recommended to run it on a single node at a time.
> Defragmentation to a live member blocks the system from reading and writing data while rebuilding its state.

Once the defragmentation is complete, the database size will match closely to the in use size:

```bash
$ talosctl -n <CP1> etcd status
NODE         MEMBER             DB SIZE   IN USE             LEADER             RAFT INDEX   RAFT TERM   RAFT APPLIED INDEX   LEARNER   ERRORS
172.20.0.2   a49c021e76e707db   4.5 MB    4.5 MB (100.00%)   ecebb05b59a776f1   56065        4           56065                false
```

## Snapshotting

Regular backups of `etcd` database should be performed to ensure that the cluster can be restored in case of a failure.
This procedure is described in the [disaster recovery]({{< relref "disaster-recovery" >}}) guide.
