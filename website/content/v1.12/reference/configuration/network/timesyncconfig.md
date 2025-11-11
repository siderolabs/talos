---
description: TimeSyncConfig is a config document to configure time synchronization (NTP).
title: TimeSyncConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: TimeSyncConfig
# Specifies NTP configuration to sync the time over network.
ntp:
    # Specifies time (NTP) servers to use for setting the system time.
    servers:
        - pool.ntp.org
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: TimeSyncConfig
# Specific PTP (Precision Time Protocol) configuration to sync the time over PTP devices.
ptp:
    # description: |
    devices:
        - /dev/ptp0
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Indicates if the time synchronization is enabled for the machine.<br>Defaults to `true`.  | |
|`bootTimeout` |Duration |Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.<br>NTP sync will be still running in the background.<br>Defaults to "infinity" (waiting forever for time sync)  | |
|`ntp` |<a href="#TimeSyncConfig.ntp">NTPConfig</a> |Specifies NTP configuration to sync the time over network.<br>Mutually exclusive with PTP configuration.  | |
|`ptp` |<a href="#TimeSyncConfig.ptp">PTPConfig</a> |Specific PTP (Precision Time Protocol) configuration to sync the time over PTP devices.<br>Mutually exclusive with NTP configuration.  | |




## ntp {#TimeSyncConfig.ntp}

NTPConfig represents a NTP server configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`servers` |[]string |Specifies time (NTP) servers to use for setting the system time.<br>Defaults to `time.cloudflare.com`.  | |






## ptp {#TimeSyncConfig.ptp}

PTPConfig represents a PTP (Precision Time Protocol) configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`devices` |[]string |description: |<br>    A list of PTP devices to sync with (e.g. provided by the hypervisor).<br><br>    A PTP device is typically represented as a character device file in the /dev directory,<br>   such as /dev/ptp0 or /dev/ptp_kvm. These devices are used to synchronize the system time<br>    with an external time source that supports the Precision Time Protocol.<br>  | |








