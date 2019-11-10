<template>
  <div id="content" class="content mb-4 section-docs">
    <div class="c-rich-text">
      <div class="flex justify-center">
        <h1>Roadmap</h1>
      </div>
      <div class="flex flex-col justify-between">
        <div class="my-6">
          <h2>v0.4</h2>
          <p>
            Emphasis on enterprise features, stability, and API expansion.
          </p>
          <ul>
            <li>Addition of more APIs</li>
            <li>
              RBAC
              <ul>
                <li>PKI</li>
                <li>OIDC</li>
              </ul>
            </li>
            <li>Logging</li>
            <li>E2E and Conformance testing for all supported platforms</li>
            <li>Test coverage of 40%</li>
          </ul>
        </div>
        <div class="my-6">
          <h2>v0.5</h2>
          <p>Emphasis on security and testing.</p>
          <ul>
            <li>Auditing</li>
            <li>Security reporting and patch policy</li>
            <li>Upgrade Controller graduated to beta</li>
            <li>Talos conformance release</li>
            <li>Test coverage of 50%</li>
          </ul>
        </div>
        <div class="my-6">
          <h2>v0.6</h2>
          <p>Emphasis on automation of operational tasks.</p>
          <ul>
            <li>Automated certificate rotation</li>
            <li>Test coverage of 60%</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  head: {
    bodyAttrs: {
      class: 'kind-section section-docs'
    }
  }
}
</script>

<style scoped>
ul {
  padding-left: 40px;
}
</style>
