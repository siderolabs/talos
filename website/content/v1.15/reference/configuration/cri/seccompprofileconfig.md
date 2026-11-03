---
description: |
    SeccompProfileConfig declares a named seccomp profile.
    SeccompProfileConfig declares a seccomp profile which can be referenced by name.

    Every profile is written to `/var/lib/kubelet/seccomp/profiles/<name>`, so Kubernetes pods
    can use it with `securityContext.seccompProfile.type: Localhost` and
    `localhostProfile: profiles/<name>`.

    This document replaces the deprecated `.machine.seccompProfiles` field.
title: SeccompProfileConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SeccompProfileConfig
name: audit.json # Name of the seccomp profile.
# The seccomp profile, in the OCI runtime specification `LinuxSeccomp` format.
profile:
    defaultAction: SCMP_ACT_LOG
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the seccomp profile.<br><br>The name is used as the profile's file name, so it must be at most 255 characters long<br>and may only contain ASCII letters, digits, `.`, `_` and `-`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: audit.json
{{< /highlight >}}</details> | |
|`profile` |Unstructured |The seccomp profile, in the OCI runtime specification `LinuxSeccomp` format.<br><br>This is the same JSON format Kubernetes `Localhost` seccomp profiles use, expressed<br>as YAML. `defaultAction` is required.  | |






