---
title: Configuration
description: Talos Linux machine configuration reference.
---

Talos Linux machine is fully configured via a single YAML file called *machine configuration*.

The file might contain one or more configuration documents separated by `---` (three dashes) lines.
At the moment, majority of the configuration options are within the [v1alpha1]({{< relref "./v1alpha1" >}}) document, so
this is the only mandatory document in the configuration file.

Configuration documents might be named (contain a `name:` field) or unnamed.
Unnamed documents can be supplied to the machine configuration file only once, while named documents can be supplied multiple times with unique names.

The `v1alpha1` document has its own (legacy) structure, while every other document has the following set of fields:

```yaml
apiVersion: v1alpha1 # version of the document
kind: NetworkRuleConfig # type of document
name: rule1 # only for named documents
```

This section contains the configuration reference, to learn more about Talos Linux machine configuration management, please see:

* [quick guide to configuration generation]({{< relref "../../introduction/getting-started#configure-talos-linux" >}})
* [configuration management in production]({{< relref "../../introduction/prodnotes#configure-talos" >}})
* [configuration patches]({{< relref "../../talos-guides/configuration/patching" >}})
* [editing live machine configuration]({{< relref "../../talos-guides/configuration/editing-machine-configuration" >}})
