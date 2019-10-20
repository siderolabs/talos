<template>
  <div class="c-rich-text">
    <div class="flex flex-col justify-around">
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <h1>What is Talos?</h1>
        <div>
          <p>
            Talos is a made up of a few pieces. The first piece is an
            API-driven, purpose-built OS, based on Linux. Its entire purpose is
            to host a Kubernetes cluster, so it is not a general purpose OS. The
            OS consists of the Linux kernel, a simplified init system, and a
            series of components that run in containers on the system, managing
            things like connections between the nodes, routes and networking,
            and serving APIs. It doesn't have SSH or a package manager. Instead,
            all of the management of Talos happens over an API.
          </p>

          <p>
            The second piece of Talos is <tt>osctl</tt>: the command-line
            interface to manage your Talos-based Kubernetes cluster. It lets you
            do all of the basic system administration on your cluster. You can
            view process lists, download log files, and so forth. It's designed
            specifically to help you debug problems on the cluster during
            development and testing.
          </p>
        </div>
      </div>
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <h1>How is Talos different than CoreOS/RancherOS/Linuxkit?</h1>
        <div>
          <p>
            Talos shares a lot of attributes with these systems, but there are
            some important differences. Talos integrates tightly with
            Kubernetes, and is not meant to be a general-purpose operating
            system. The most important difference is that Talos is fully
            controlled by an API via a gRPC interface, instead of an ordinary
            shell. We don't ship SSH, and there is no console access. Removing
            components such as these has allowed us to dramatically reduce the
            footprint of Talos, and in turn, improve a number of other areas
            like security, predictability, reliability, and consistency across
            platforms. It's a big change from how operating systems have been
            managed in the past, but we believe that API-driven OSes are the
            future.
          </p>
        </div>
      </div>
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <h1>Why no shell or SSH?</h1>
        <div>
          <p>
            Since Talos is fully API-driven, all maintenance and debugging
            operations should be possible via the OS API. We would like for
            Talos users to start thinking about what a "machine" is in the
            context of a Kubernetes cluster. That is, that a Kubernetes
            <i>cluster</i> can be thought of as one massive machine, and the
            <i>nodes</i> are merely additional, undifferentiated resources. We
            don't want humans to focus on the <i>nodes</i>, but rather on the
            <i>machine</i> that is the Kubernetes cluster. Should an issue arise
            at the node level, <tt>osctl</tt> should provide the necessary
            tooling to assist in the identification, debugging, and remedation
            of the issue. However, the API is based on the Principle of Least
            Privilege, and exposes only a limited set of methods. We envision
            Talos being a great place for the application of
            <a href="https://en.wikipedia.org/wiki/Control_theory"
              >control theory</a
            >
            in order to provide a self-healing platform.
          </p>
        </div>
      </div>
      <div class="text-left w-3/4 mx-20 py-10 px-20">
        <h1>Why the name "Talos"?</h1>
        <div>
          <p>
            Talos was an automaton created by the Greek God of the forge to
            protect the island of Crete. He would patrol the coast and enforce
            laws throughout the land. We felt it was a fitting name for a
            security focused operating system designed to run Kubernetes.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
