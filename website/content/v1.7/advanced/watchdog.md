---
title: "Watchdog Timers"
description: "Using hardware watchdogs to workaround hardware/software lockups."
---

Talos Linux now supports hardware watchdog timers configuration.
Hardware watchdog timers allow to reset (reboot) the system if the software stack becomes unresponsive.
Please consult your hardware/VM documentation for the availability of the hardware watchdog timers.

## Configuration

To discover the available watchdog devices, run:

```shell
$ talosctl ls /sys/class/watchdog/
NODE         NAME
172.20.0.2   .
172.20.0.2   watchdog0
172.20.0.2   watchdog1
```

The implementation of the watchdog device can be queried with:

```shell
$ talosctl read /sys/class/watchdog/watchdog0/identity
i6300ESB timer
```

To enable the watchdog timer, patch the machine configuration with the following:

```yaml
# watchdog.yaml
apiVersion: v1alpha1
kind: WatchdogTimerConfig
device: /dev/watchdog0
timeout: 5m
```

```shell
talosctl patch mc -p @watchdog.yaml
```

Talos Linux will set up the watchdog time with a 5-minute timeout, and it will keep resetting the timer to prevent the system from rebooting.
If the software becomes unresponsive, the watchdog timer will expire, and the system will be reset by the watchdog hardware.

## Inspection

To inspect the watchdog timer configuration, run:

```shell
$ talosctl get watchdogtimerconfig
NODE         NAMESPACE   TYPE                  ID      VERSION   DEVICE           TIMEOUT
172.20.0.2   runtime     WatchdogTimerConfig   timer   1         /dev/watchdog0   5m0s
```

To inspect the watchdog timer status, run:

```shell
$ talosctl get watchdogtimerstatus
NODE         NAMESPACE   TYPE                  ID      VERSION   DEVICE           TIMEOUT
172.20.0.2   runtime     WatchdogTimerStatus   timer   1         /dev/watchdog0   5m0s
```

Current status of the watchdog timer can also be inspected via Linux sysfs:

```shell
$ talosctl read /sys/class/watchdog/watchdog0/state
active
```
