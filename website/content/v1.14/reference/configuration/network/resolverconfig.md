---
description: ResolverConfig is a config document to configure DNS resolving.
title: ResolverConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ResolverConfig
# A list of nameservers (DNS servers) to use for resolving domain names.
nameservers:
    - address: 1.1.1.1 # The IP address of the nameserver.
    - address: ff08::1 # The IP address of the nameserver.
# Configuration for search domains (in /etc/resolv.conf).
searchDomains:
    # A list of search domains to be used for DNS resolution.
    domains:
        - example.com
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ResolverConfig
# Configuration for search domains (in /etc/resolv.conf).
searchDomains:
    disableDefault: true # Disable default search domain configuration from hostname FQDN.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ResolverConfig
# Configuration for host DNS resolver.
hostDNS:
    enabled: true # Enable host DNS caching resolver.
    forwardKubeDNSToHost: true # Use the host DNS resolver as upstream for Kubernetes CoreDNS pods.
    resolveMemberNames: true # Resolve member hostnames using the host DNS resolver.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`nameservers` |<a href="#ResolverConfig.nameservers.">[]NameserverConfig</a> |A list of nameservers (DNS servers) to use for resolving domain names.<br><br>Nameservers are used to resolve domain names on the host, and they are also<br>propagated to Kubernetes DNS (CoreDNS) for use by pods running on the cluster.<br><br>This overrides any nameservers obtained via DHCP or platform configuration.<br>Default configuration is to use 1.1.1.1 and 8.8.8.8 as nameservers.  | |
|`searchDomains` |<a href="#ResolverConfig.searchDomains">SearchDomainsConfig</a> |Configuration for search domains (in /etc/resolv.conf).<br><br>The default is to derive search domains from the hostname FQDN.  | |
|`hostDNS` |<a href="#ResolverConfig.hostDNS">HostDNSConfig</a> |Configuration for host DNS resolver.<br><br>This configures a local DNS caching resolver on the host to improve DNS resolution performance and reliability.  | |




## nameservers[] {#ResolverConfig.nameservers.}

NameserverConfig represents a single nameserver configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Addr |The IP address of the nameserver. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 10.0.0.1
{{< /highlight >}}</details> | |






## searchDomains {#ResolverConfig.searchDomains}

SearchDomainsConfig represents search domains configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`domains` |[]string |A list of search domains to be used for DNS resolution.<br><br>Search domains are appended to unqualified domain names during DNS resolution.<br>For example, if "example.com" is a search domain and a user tries to resolve<br>"host", the system will attempt to resolve "host.example.com".<br><br>This overrides any search domains obtained via DHCP or platform configuration.<br>The default configuration derives the search domain from the hostname FQDN.  | |
|`disableDefault` |bool |Disable default search domain configuration from hostname FQDN.<br><br>When set to true, the system will not derive search domains from the hostname FQDN.<br>This allows for a custom configuration of search domains without any defaults.  | |






## hostDNS {#ResolverConfig.hostDNS}

HostDNSConfig represents host DNS configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable host DNS caching resolver.<br><br>When enabled, a local DNS caching resolver is deployed on the host to improve DNS resolution performance and reliability.<br>Upstream DNS servers for the host resolver are configured using the `nameservers` field in this config document.  | |
|`forwardKubeDNSToHost` |bool |Use the host DNS resolver as upstream for Kubernetes CoreDNS pods.<br><br>When enabled, CoreDNS pods use host DNS server as the upstream DNS (instead of<br>using configured upstream DNS resolvers directly).  | |
|`resolveMemberNames` |bool |Resolve member hostnames using the host DNS resolver.<br><br>When enabled, cluster member hostnames and node names are resolved using the host DNS resolver.<br>This requires service discovery to be enabled.  | |








