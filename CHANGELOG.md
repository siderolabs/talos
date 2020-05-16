
<a name="v0.6.0-alpha.0"></a>
## [v0.6.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.5.0-beta.0...v0.6.0-alpha.0) (2020-05-16)

### Chore

* fix nits in the events code

### Fix

* run machined API as a service
* respect nameservers when using docker cluster
* update Events API response type to match proxying conventions
* register event service with router


<a name="v0.5.0-beta.0"></a>
## [v0.5.0-beta.0](https://github.com/talos-systems/talos/compare/v0.5.0-alpha.2...v0.5.0-beta.0) (2020-05-13)

### Chore

* serialize firecracker e2e tests
* pin markdown linting libraries
* use clusterctl and v1alpha3 providers for tests
* fix prototool lint

### Docs

* add a sitemap and Netlify redirects
* adjust docs layouts and add tables of contents
* update copyright date
* backport intro text to 0.3 and 0.4 docs
* fix netlify deep linking for 0.5 docs by generating fallback routes
* add 0.5 pre-release docs, add linkable anchors, other fixes

### Feat

* add events API
* add support for file scheme
* enable rpfilter
* add bootstrap API
* add recovery API
* allow dual-stack support with bootkube wrapper

### Fix

* refactor client creation API
* update kernel package
* write machined RPC logs to file
* clean up docs page scripts in preparation for 0.5 docs
* ipv6 static default gateway not set if gateway is a LL unicast address

### Refactor

* remove warning about missing boot partition

### Release

* **v0.5.0-beta.0:** prepare release

### Test

* add node name to error messages in RebootAllNodes
* stabilize tests by bumping timeouts

