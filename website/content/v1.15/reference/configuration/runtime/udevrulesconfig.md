---
description: UdevRulesConfig is a udev rules config document.
title: UdevRulesConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: UdevRulesConfig
# Custom udev rules.
rules:
    - SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rules` |[]string |Custom udev rules.  | |






