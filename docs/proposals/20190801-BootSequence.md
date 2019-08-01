# Proposal: Boot Sequence

Author: Andrew Rynhard

## Abstract

Ensuring proper order of tasks in boot and shutdown events is currently a problem.
This document proposes a boot sequence management system that can be used to boot and shutdown a node in the proper order.

## Background

The main routine of `machined` was very script-like in the sense that it was a very long sequence of conditionals and functions.
Lacking any kind of abstraction, a lot of code was repeated, conditionals were all over the place and hard to make sense of.
This poses problems when we start to look at what it would take to bring a node back to its original state of having zero mounts, networking, processes (except PID 1 of course), etc.
Undoing what is done only exacerbates all the previously mentioned problems.

## Proposal

This proposal builds on the notion of phases, tasks, and services by outlining yet another abstraction that ties these together.
I propopse an `struct` aptly named `BootSequencer`.
The definition might look something like the following:

```go
const (
    MaxLevel = 5
)

type Status int

const (
    Pass Status = iota
    Fail
)

type Level struct {
    status Status
    phases []phase.Phase
}

type Levels []Level

type BootSequencer struct {
    phase.Runner

    levels Levels
}

func (b *BootSequencer) Register(p phase.Phase, l int) error {
    levels[l] = append(levels[l], p)
}

var boot *BootSequencer

func init {
    boot = &BootSequencer{
        Runner: phase.NewRunner(),
        levels: make([]phase.Phase, 0, MaxLevel)
    }

    for i := 0; i < MaxLevel; i++ {
        boot.levels = append(boot.levels, make([]phase.Phase, 0))
    }
}

func main() {
    // Defer all shutdown logic.
    defer boot.Shutdown()
    // Run all phases.
    boot.Run()
    // Wait on channels.
    boot.Wait()
}
```

Then a task can register itself with a specific phase like so:

```go
type Phase0Task struct {

}

func init {
    boot.Register(&Phase0Task{}, 0)
}
```

Notice that `phase.Runner` is embedded.
This serves to enforce the idea that a `BootSequencer` is a wrapper around a `phase.Runner`, that adds some extra functionality on top of it.
The `BootSequencer` will perform the following tasks:

- register phases and build the execution order
- create any event handling channels
- run all phases to bring the node to a running state
- defer any functions that might be needed on a shutdown from a panic, shutdown/reboot request via API/ACPI

The `BootSequencer` could, one day, house a global event bus that is shared across phases, tasks, and services.

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Compatibility

This change builds on existing types and is compatiable since it will be just another level of abstraction that doesn't introduce any features that fundamentally break compatibility.

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
