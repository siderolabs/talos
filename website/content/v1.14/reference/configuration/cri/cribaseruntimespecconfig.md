---
description: CRIBaseRuntimeSpecConfig configures the base OCI runtime specification
    for CRI containers.
title: CRIBaseRuntimeSpecConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CRIBaseRuntimeSpecConfig
# Overrides for the default OCI runtime specification used by CRI containers.
overrides:
    process:
        rlimits:
            - hard: 1024
              soft: 1024
              type: RLIMIT_NOFILE
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`overrides` |Unstructured |Overrides for the default OCI runtime specification used by CRI containers.<br><br>This document is mutually exclusive with the deprecated<br>`.machine.baseRuntimeSpecOverrides` field.<br><br>Strategic merge patches replace this overrides object as a whole, so<br>reapplying the same document is idempotent.<br><br>Applying, updating, or removing these overrides restarts CRI automatically.<br>A machine reboot is not required.  | |






