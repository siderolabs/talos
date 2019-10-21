<template>
  <div id="landing-page" class="c-rich-text">
    <div class="w-1/2 h-auto mx-auto mt-10 mb-20">
      <div class="flex flex-col justify-center">
        <div class="text-center">
          <h1>What is Talos?</h1>
        </div>
        <div class="text-center">
          <div>
            Talos is a modern OS designed to be secure, immutable, and minimal.
            Its purpose is to host Kubernetes clusters, so it is tightly
            integrated with Kubernetes. Talos is based on the Linux kernel, and
            supports most cloud platforms, bare metal, and most virtualization
            platforms. All system management is done via an API, and there is no
            shell or interactive console.
          </div>
        </div>
      </div>
    </div>

    <div class="w-full h-64 mx-auto mb-20">
      <div class="flex flex-col justify-center">
        <div class="text-center">
          <h1>Why Talos?</h1>
        </div>
        <div class="flex justify-center">
          <div class="text-center w-1/4">
            <h2>Security</h2>
            <div>
              Talos reduces your attack surface by practicing the Principle of
              Least Privilege (PoLP) and by securing the API with mutual TLS
              (mTLS) authentication.
            </div>
          </div>
          <div class="text-center w-1/4">
            <h2>Predictability</h2>
            <div>
              Talos eliminates unneeded variables and reduces unknown factors in
              your environment by employing immutable infrastructure ideology.
            </div>
          </div>
          <div class="text-center w-1/4">
            <h2>Evolvability</h2>
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
      <div class="flex flex-col justify-center">
        <div class="text-center">
          <h1>Built with Modern Technology</h1>
        </div>
        <div class="flex justify-center">
          <div class="flex items-center m-6 p-2">
            <a href="https://www.musl-libc.org/" target="_blank"
              ><img class="logo logo-musl" src="/images/musl-logo.png" alt=""
            /></a>
          </div>
          <div class="flex items-center m-6 p-2">
            <a href="https://golang.org/" target="_blank"
              ><img class="logo" src="/images/go-logo.png" alt=""
            /></a>
          </div>
          <div class="flex items-center m-6 p-2">
            <a href="https://grpc.io/" target="_blank"
              ><img class="logo" src="/images/grpc-logo.png" alt=""
            /></a>
          </div>
          <div class="flex items-center m-6 p-2">
            <a href="https://containerd.io/" target="_blank"
              ><img class="logo" src="/images/containerd-logo.png" alt=""
            /></a>
          </div>
        </div>
      </div>
    </div>

    <div class="w-3/4 h-auto mx-auto mb-20">
      <div class="flex flex-col justify-center">
        <div class="text-center">
          <h1>Features</h1>
        </div>
        <div class="flex justify-center">
          <div class="text-center w-full px-2 py-2 m-2">
            <h2>Minimal</h2>
            <div>
              Talos is a minimalistic distribution that consists of only a
              handful of binaries and shared libraries. Just enough to run
              containerd and a small set of system services. This aligns with
              NIST's recommendation in the
              <a
                href="https://www.nist.gov/publications/application-container-security-guide"
                target="_blank"
              >
                Application Container Security Guide </a
              >.
            </div>
          </div>
          <div class="text-center w-full px-2 py-2 m-2">
            <h2>Hardened</h2>
            <div>
              There are a number of ways that Talos provides added hardening
              <ul>
                <li>
                  employs the recommended configuration and runtime settings
                  outlined in the
                  <a
                    href="https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project"
                    target="_blank"
                    >Kernel Self Protection Project</a
                  >
                </li>
                <li>
                  enables mutual TLS for the API
                </li>
                <li>
                  enforces the settings and configurations described in the
                  <a
                    href="https://www.cisecurity.org/benchmark/kubernetes/"
                    target="_blank"
                    >CIS</a
                  >

                  guidelines
                </li>
              </ul>
            </div>
          </div>
          <div class="text-center w-full px-2 py-2 m-2">
            <h2>Immutable</h2>
            <div>
              Talos improves its security posture further by mounting the root
              filesystem as read-only and removing any host-level access by
              traditional means such as a shell and SSH.
            </div>
          </div>
          <div class="text-center w-full px-2 py-2 m-2">
            <h2>Current</h2>
            <div>
              Stay current with our commitment to an
              <tt>n-1</tt>
              adoption rate of upstream Kubernetes. Additionally, the latest LTS
              Linux kernel will always be used.
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="flex flex-col items-center my-12">
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

    <div class="flex flex-col items-center my-12">
      <img class="cncf" src="/images/cncf-color.png" alt="" />
      <p>
        We are a
        <a href="https://cncf.io" target="_blank"
          >Cloud Native Computing Foundation</a
        >
        member.
      </p>
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

.cncf {
  width: 250px;
  height: 39px;
}

.certified {
  height: 200px;
  width: auto;
}
</style>
