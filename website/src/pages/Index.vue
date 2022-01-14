<template>
  <Layout>
    <div id="landing-page" class="c-rich-text">
      <div class="w-auto lg:w-3/4 mx-auto md:mt-10 mb-8 px-4">
        <div class="flex flex-wrap justify-center items-center">
          <div class="text-xl text-gray-700 lg:w-1/2">
            <div class="text-center py-8 m-0">
              <h1>What is Talos Linux?</h1>
            </div>

            <p>
              Talos Linux is Linux designed for Kubernetes – secure, immutable, and minimal.
            </p>

            <p>
              Talos Linux supports cloud platforms, bare metal, and virtualization platforms. All system management is done via an API: there is no ssh, shell or interactive console.
            </p>
            <p>
              Talos Linux is production ready, and supports some of the largest Kubernetes clusters in the world.
            </p>

            <div class="py-4 flex">
              <h3 class="flex-1  text-gray-600 mr-2">
                It only takes  3 minutes to launch a  Talos cluster on your
                laptop inside Docker.
              </h3>
              <div class="flex-1 text-center pb-4 m-0">
                <a href="/docs/latest/introduction/quickstart/">
                  <button class="teal-cta-button">Try it now</button></a
                >
              </div>
            </div>
          </div>
          <div class="w-2/3 lg:w-1/2 lg:pl-8">
            <g-image src="/images/talos-arch.svg" />
          </div>
        </div>
      </div>

      <div class="w-3/4 mx-auto">
        <div class="justify-center">
          <div class="text-center md:py-8">
            <h1>Why Talos Linux?</h1>
          </div>
          <div class="md:grid grid-columns-3 grid-gap-12 justify-center">
            <div class="px-4">
              <h2 class="text-center">Security</h2>
              <div>
                Talos reduces your attack surface. It's minimal, hardened and immutable. All API access is secured with mutual TLS
                (mTLS) authentication.
              </div>
            </div>
            <div class="px-4">
              <h2 class="text-center">Predictability</h2>
              <div>
                Talos eliminates configuration drift, reduces unknown factors
                 by employing immutable infrastructure
                ideology, and delivers atomic updates.
              </div>
            </div>
            <div class="px-4">
              <h2 class="text-center">Evolvability</h2>
              <div>
                Talos simplifies your architecture, increases your agility, and makes always delivers current stable Kubernetes and Linux versions.
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="w-3/4 mx-auto mb-20 pt-4">
        <div class="flex flex-col justify-center">
          <div class="text-center">
            <h2>API Driven</h2>
            <Terminal></Terminal>
          </div>
        </div>
      </div>

      <div class="w-1/2 h-auto mx-auto">
        <div class="text-center pt-6">
          <h1>Built with Modern Technology</h1>
        </div>
        <div class="flex flex-wrap md:flex-no-wrap justify-center">
          <div class="flex items-center lg:m-6 p-2">
            <a href="https://www.musl-libc.org/" target="_blank"
              ><img class="logo logo-musl" src="/images/musl-logo.png" alt=""
            /></a>
          </div>
          <div class="flex items-center lg:m-6 p-2">
            <a href="https://golang.org/" target="_blank"
              ><img class="logo" src="/images/go-logo.png" alt=""
            /></a>
          </div>
          <div class="flex items-center lg:m-6 p-2">
            <a href="https://grpc.io/" target="_blank"
              ><img class="logo" src="/images/grpc-logo.png" alt=""
            /></a>
          </div>
          <div class="flex items-center lg:m-6 p-2">
            <a href="https://containerd.io/" target="_blank"
              ><img class="logo" src="/images/containerd-logo.png" alt=""
            /></a>
          </div>
        </div>
      </div>

      <div class="w-3/4 h-auto mx-auto pb-8">
        <div class="text-center">
          <h1>Features</h1>
        </div>
        <div class="md:grid grid-columns-3 grid-gap-12 justify-center">
          <div class="w-full">
            <h2 class="text-center">Minimal</h2>
            <p>
              Talos consists of only a handful of binaries and shared libraries:
              just enough to run containerd and a small set of system services.
            </p>

            <p>
              This aligns with NIST's recommendation in the
              <a
                href="https://www.nist.gov/publications/application-container-security-guide"
                target="_blank"
              >
                Application Container Security Guide</a
              >.
            </p>
          </div>
          <div>
            <h2 class="text-center">Hardened</h2>
            <div>
              <p>Hardened by design:</p>
              <ul>
                <li>
                  Built with the
                  <a
                    href="https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project"
                    target="_blank"
                    >Kernel Self Protection Project</a
                  >
                  configuration recommendations.
                </li>
                <li>All access to the API is secured with Mutual TLS.</li>
                <li>
                  Settings and configuration described in the
                  <a
                    href="https://www.cisecurity.org/benchmark/kubernetes/"
                    target="_blank"
                    >CIS</a
                  >
                  guidelines are applied by default.
                </li>
              </ul>
            </div>
          </div>
          <div>
            <h2 class="text-center">Immutable</h2>
            <p>
              Talos improves security further by mounting the root
              filesystem as read-only and removing any host-level such as a shell and SSH.
            </p>
          </div>
        </div>

        <div
          class="md:grid grid-columns-2 grid-gap-12 justify-center w-2/3 mx-auto"
        >
          <div>
            <h2 class="text-center">Ephemeral</h2>
            <p>
              Talos runs in memory from a SquashFS, and persists nothing,
              leaving the primary disk entirely to Kubernetes.
            </p>
          </div>
          <div>
            <h2 class="text-center">Current</h2>
            <p>
              Delivers the latest stable
              versions of Kubernetes and Linux.
            </p>
          </div>
        </div>
      </div>

      <div class="flex pb-8 justify-center">
        <a
          href="https://landscape.cncf.io/category=certified-kubernetes-installer&format=card-mode&grouping=category"
          target="_blank"
        >
          <img
            class="h-48 mx-auto"
            src="/images/certified-kubernetes-color.png"
            alt="Certified Kubernetes Logo"
        /></a>
      </div>
    </div>
  </Layout>
</template>

<script>
import Terminal from "~/components/Terminal.vue";

export default {
  name: "Index",
  components: {
    Terminal,
  },
};
</script>

<style scoped>
ul {
  list-style-type: disc;
}

.logo {
  display: block;
  width: 189px;
  height: auto;
  margin: auto;
}

.logo-musl {
  width: 107px;
}

.certified {
  @apply m-5;
  height: auto;
  width: 400px;
}

.c-rich-text ul {
  list-style-position: inside;
}

.teal-cta-button {
  @apply bg-logo-teal text-white font-bold py-2 px-4 rounded;
}

.teal-cta-button:hover {
  @apply bg-logo-teal-darker;
}
</style>
