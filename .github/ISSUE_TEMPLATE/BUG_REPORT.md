---
name: Bug Report
about: Report an observed failure with reproduction steps and diagnostic evidence.
title: ""
labels: ""
assignees: ""
---

## Bug Report

Tell us what failed and include logs and steps we can follow to reproduce it.
We need evidence from the failure, not a long code analysis or an AI-generated explanation.
A proposed fix or a test of a suspected cause is not a substitute.
Keep it short, distinguish what you saw from what you suspect, and only include commands and output you have checked yourself.
You do not need to find the cause or write a fix.
We may close reports that do not give us enough information to investigate.

### What happened? What did you expect?

<!-- Describe the actual failure and expected behavior. Include the time (with timezone) and affected nodes. -->

### Steps to reproduce

<!--
Include the setup, exact commands, and the smallest configuration or manifests needed. Remove secrets.
How often does it happen? If it happened only once or you cannot reproduce it, say so and share the logs you have.
If you deliberately broke something to trigger the failure, explain what you changed. That alone does not prove what caused the original incident.
-->

### Logs and diagnostics

Attach a support bundle from the affected nodes if you can:

```sh
talosctl support --nodes <affected-node-IP>
```

Collect it while the failure is happening and before rebooting, if possible.
If you cannot collect a bundle, tell us why and attach whatever logs you have. Support bundles are encrypted.

For boot or shutdown failures, or when a node is unreachable, include console logs from serial, the hypervisor, or BMC/iDRAC/iLO.
**Text logs are preferred.** If you cannot capture text, attach a readable video of the failure, or photos if the error stays on screen.
Include what happened before the error. Use fenced code blocks for short excerpts and attach long logs as files.

Remove credentials from logs and configuration snippets you attach separately.
Do not attach unredacted machine configurations, talosconfig, or kubeconfig files.

### Environment

- Talos version (client and server): [`talosctl version --nodes <affected-node-IP>`]
- Kubernetes version (if relevant): [`kubectl version`]
- Platform (hardware model, hypervisor, or cloud):
- System extensions and relevant configuration:
- Recent changes and last known working version (if known):
