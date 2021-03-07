---
title: "Architecture"
weight: 3
---

Talos is designed to be **atomic** in _deployment_ and **modular** in _composition_.

It is atomic in the sense that the entirety of Talos is distributed as a
single, self-contained image, which is versioned, signed, and immutable.

It is modular in the sense that it is composed of many separate components
which have clearly defined gRPC interfaces which facilitate internal flexibility
and external operational guarantees.

There are a number of components which comprise Talos.
All of the main Talos components communicate with each other by gRPC, through a socket on the local machine.
This imposes a clear separation of concerns and ensures that changes over time which affect the interoperation of components are a part of the public git record.
The benefit is that each component may be iterated and changed as its needs dictate, so long as the external API is controlled.
This is a key component in reducing coupling and maintaining modularity.
