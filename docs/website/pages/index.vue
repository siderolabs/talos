<template>
  <div id="landing-page" class="c-rich-text">
    <div class="w-auto md:w-1/2 h-auto mx-auto mt-10 mb-20 px-4">
      <div class="text-center p-0 m-0">
        <h1>What is Talos?</h1>
      </div>
      <div class="flex flex-wrap justify-center items-center">
        <div class="px-4 md:px-2 md:w-3/4">
          <p>
            Talos is a modern OS designed to be secure, immutable, and minimal.
            Its purpose is to host Kubernetes clusters, so it is tightly
            integrated with Kubernetes.
          </p>
          <p>
            Talos is based on the Linux kernel, and supports most cloud
            platforms, bare metal, and most virtualization platforms. All system
            management is done via an API, and there is no shell or interactive
            console.
          </p>
        </div>
        <div class="w-1/6">
          <a
            href="https://landscape.cncf.io/category=certified-kubernetes-installer&format=card-mode&grouping=category"
            target="_blank"
          >
            <img
              class="certified"
              src="/images/certified-kubernetes-color.png"
              alt=""
          /></a>
        </div>
      </div>
    </div>

    <div class="w-3/4 h-64 mx-auto pb-20">
      <div class="justify-center">
        <div class="text-center">
          <h1>Why Talos?</h1>
        </div>
        <div class="md:grid grid-columns-3 grid-gap-12 justify-center">
          <div class="px-4">
            <h2 class="text-center">Security</h2>
            <div>
              Talos reduces your attack surface by practicing the Principle of
              Least Privilege (PoLP) and by securing the API with mutual TLS
              (mTLS) authentication.
            </div>
          </div>
          <div class="px-4">
            <h2 class="text-center">Predictability</h2>
            <div>
              Talos eliminates unneeded variables and reduces unknown factors in
              your environment by employing immutable infrastructure ideology.
            </div>
          </div>
          <div class="px-4">
            <h2 class="text-center">Evolvability</h2>
            <div>
              Talos simplifies your architecture and increases your ability to
              easily accommodate future changes.
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="w-3/4 mx-auto mb-20">
      <div class="flex flex-col justify-center">
        <div class="text-center">
          <h2>API Driven</h2>
          <Terminal></Terminal>
        </div>
      </div>
    </div>

    <div class="w-1/2 h-auto mx-auto mb-20">
      <div class="text-center">
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

    <div class="w-3/4 h-auto mx-auto pb-8 mb-20">
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
            <p>Talos is hardened by design and configuration:</p>
            <ul class="ml-8">
              <li>
                We follow the
                <a
                  href="https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project"
                  target="_blank"
                  >Kernel Self Protection Project</a
                >
                configuration recommendations.
              </li>
              <li>
                We enable mutual TLS for the API.
              </li>
              <li>
                We enforce the settings and configurations described in the
                <a
                  href="https://www.cisecurity.org/benchmark/kubernetes/"
                  target="_blank"
                  >CIS</a
                >
                guidelines.
              </li>
            </ul>
          </div>
        </div>
        <div>
          <h2 class="text-center">Immutable</h2>
          <div>
            Talos improves its security posture further by mounting the root
            filesystem as read-only and removing any host-level access by
            traditional means such as a shell and SSH.
          </div>
        </div>
        <div>
          <h2 class="text-center">Ephemeral</h2>
          <div>
            Talos runs in memory from a SquashFS, and persists nothing, leaving
            the primary disk entirely to Kubernetes.
          </div>
        </div>
        <div>
          <h2 class="text-center">Current</h2>
          <div>
            We are committed to an <tt>n-1</tt> adoption rate of upstream
            Kubernetes, and the latest LTS Linux kernel will always be used.
          </div>
        </div>
        <div>
          <img
            src="https://placeholder.pics/svg/300/DEDEDE/555555/some%20nifty%20image%20here"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Terminal from '~/components/Terminal.vue'

export default {
  name: 'Index',
  components: {
    Terminal
  }
}
</script>

<style>
#landing-page {
  height: 100%;
  background: url(~assets/images/chips_bg_v1.svg) no-repeat center center fixed;
  -webkit-background-size: contain;
  -moz-background-size: contain;
  -o-background-size: contain;
  background-size: contain;
  background-position: center bottom;
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
</style>
