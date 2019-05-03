# Proposal: Control Plane Bootstrapping

Author(s): [Brad Beam](@bradbeam) [Andrew Rynhard](@andrewrynhard)

Last updated: May 3 2019

## Abstract

There are limitations with the current bootstrapping process that prevent
us from implementing a clean in place upgrade. We propose that making updates
to userdata, OSD, and init that we will be able to make the control plane
bootstrap process more reliable and deterministic.

## Background

Currently we support two different types of control plane configurations -
`init/bootstrap` and `control plane`. The main difference between these two
configurations comes down to the `kubeadm` config to denote if the node should
run `kubeadm init` or `kubeadm join`.

This has the potential to be problematic when considering upgrades and
understanding which action the init node should take. In the current state
it would attempt to perform a `kubeadm init` which would break the existing
cluster.

## Proposal

### User Data

We propose adding in an `initNode` key in the userdata to denote the hostname
of the node that should perform the `kubeadm init` action.

We propose consolidating the control plane configuration to only make use of
the 'init' configuration. This is inclusive of `kubeadm.InitConfiguration`,
`kubeadm.ClusterConfiguration`, `kubeadm.KubeletConfiguration`, and
`kubeadm.KubeProxyConfiguration`.

We propose automatically generating a `kubeadm.JoinConfiguration` for the other
control plane nodes from the 'init' configuration.

### OSD

We propose adding in an additional `status` endpoint to provide visibility on
the state of the current node. This is inclusive of both the Talos control plane
as well as Kubernetes functionality.

### Init

We propose adjusting the current workflow for the kubeadm service startup to
add in additional checks to determine if the current node is an 'init' node,
if existing state is present on the node, and the status of the other control
plane nodes. With this information we can make an appropriate decision on how
the init node should behave.

## Rationale

We feel this is the simplest approach to laying the framework to build in place
upgrade functionality. In walking through the different scenarios and
implementation options, this approach seems to account for the highest level of
reliability with the lowest level of complexity introduced.

## Compatibility

This is a breaking change as it obsoletes the 'control plane' node
configuration and introduces a new key in userdata.

## Implementation

There are 3 main components to this proposal, OSD updates, init updates, and
userdata updates. Additional details can be seen above in the Proposal section.

## Open issues (if applicable)

