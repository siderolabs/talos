---
description: EnvironmentConfig is an environment config document.
title: EnvironmentConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EnvironmentConfig
# This field allows for the addition of environment variables.
variables:
    GRPC_GO_LOG_SEVERITY_LEVEL: info
    GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
    https_proxy: http://SERVER:PORT/
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`variables` |Env |This field allows for the addition of environment variables.<br>All environment variables are set on PID 1 in addition to every service.<br>Propagation of environment variables to services is done only at initial service start time.<br>To modify environment variables for services, the node must be restarted.<br>Multiple values for the same environment variable (in multiple documents) will replace previous values, with the last one taking precedence.<br>Fully removing an environment variable can only be achieved by removing it from the document and restarting the machine.<br>Environment variable names are validated, and should:<br>  - start with an uppercase letter, lowercase letter, or an underscore (_) character, and<br>  - contain only uppercase and lowercase letters, underscore (_) characters, and numbers. <details><summary>Show example(s)</summary>Environment variables definition examples.:{{< highlight yaml >}}
variables:
    GRPC_GO_LOG_SEVERITY_LEVEL: info
    GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
    https_proxy: http://SERVER:PORT/
{{< /highlight >}}{{< highlight yaml >}}
variables:
    GRPC_GO_LOG_SEVERITY_LEVEL: error
    https_proxy: https://USERNAME:PASSWORD@SERVER:PORT/
{{< /highlight >}}{{< highlight yaml >}}
variables:
    https_proxy: http://DOMAIN\USERNAME:PASSWORD@SERVER:PORT/
{{< /highlight >}}</details> |``GRPC_GO_LOG_VERBOSITY_LEVEL``<br />``GRPC_GO_LOG_SEVERITY_LEVEL``<br />``http_proxy``<br />``https_proxy``<br />``no_proxy``<br /> |






