# Proposal: [Title]

Author(s): [Andrew Rynhard](@andrewrynhard) [Brad Beam](@bradbeam)

Last updated: May 3 2019

## Abstract

Without insight into the current status/health of a Talos node, we severely limit ourselves in the quality and dependability of any automation we implement.
We propose that we need an API exposed on each node that gives detailed information about the nodes current status/heatlh.
A common set of data between masters and workers will be implemented along with data specific to masters that we can make use of in other features, like upgrades.

## Background

We have long thought that in general a status API would be useful.
Once we started to desgin upgrades it became clear that we needed an API for node status/heath that we could use to orchestrate automated, cluster-wide upgrades.

## Proposal

### Technology

We propose that we leverage the existing `osd` service and extend it to include the following APIs.

### Common Status API

The common status API is a set of metrics we require every node to have.

### Master Status API

The master status API is a set of metrics required by every node participating as a master.

## Rationale

As we think about how Talos will become a self-healing platform, we need to think about metrics we can use in our automation.
Something as simple as "is process X running" is not enough.


## Compatibility

This change introduces no incompatible changes.

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
