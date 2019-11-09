<template>
  <div class="c-rich-text">
    <div class="flex justify-center">
      <h1>Roadmap</h1>
    </div>
    <div class="flex flex-col justify-around">
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <ul>
          <h2>v0.4</h2>
          <p>Emphasis on enterprise features, stability, and API expansion.</p>
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
        </ul>
      </div>
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <ul>
          <h2>v0.5</h2>
          <p>Emphasis on security and testing.</p>
          <p></p>
          <ul>
            <li>Auditing</li>
            <li>Security reporting and patch policy</li>
            <li>Upgrade Controller graduated to beta</li>
            <li>Talos conformance release</li>
            <li>Test coverage of 50%</li>
          </ul>
        </ul>
      </div>
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <ul>
          <h2>v0.6</h2>
          <p>Emphasis on automation of operational tasks.</p>
          <p></p>
          <ul>
            <li>Automated certificate rotation</li>
            <li>Test coverage of 60%</li>
          </ul>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
ul {
  padding-left: 40px;
}
</style>
