# Proposal: Branching Model for Channels

Author(s): Andrew Rynhard

## Abstract

[A short summary of the proposal.]

## Background

[An introduction of the necessary background and the problem being solved by the proposed change.]

## Proposal

### Alpha to Beta

- Branch creation indicates beta timeline start/reset
- Bot creates a PR upon branch creation, human `/approve`s and `/lgtm`s the PR, bot merges the PR, bot tags the merged commit as the first beta
<!-- - beta branch must always be at or behind alpha -->

### Future Beta Releases

- Only critical bug fixes and security patches are eligible for cherry-pick into a release branch
- Human creates cherry-pick PR into the release branch
- A human will decide when to create the next release from a given branch (allows for multiple fixes to be brought in at a time for the next release (e.g. security fixes for two different packages that are from two different commits))
- A human will create a CHANGELOG PR with a new conventional commit type and scope (e.g. `release(20.04-beta|20.04-stable): ...`)
- Bot will tag on any merge into the branch based on
  - Does the current commit have a `release` type
    - Use the scope to determine the tag
      - Bot will use historical tags to determine increment

### Beta to Stable

- We never create a new release branch unless the previous release branch has graduated to stable
- Once a release branch is created, we are committed to pushing that feature set to stable. In other words, there must never be any two branches that are still in beta.
<!-- - stable branch must always be at or behind beta -->

### Future Stable Releases

See `Future Beta Releases`

### General Rules

- We must never have a commit be tagged more than once
- A channel must always be at or behind the previous channel
- A commit can have only have one commit (ensures `git describe --tags` is accurate)
- Alpha tags are on master branch commits
- Beta and stable tags are on release branch commits

### Detecting Graduation

- We will communicate the year, month, and channel to the bot via the conventional commit scope (e.g. `release(year.month-channel): ...`)
- `latest` to `alpha` is keyed off of a PR with a new conventional commit type and scope (e.g. `release(20.04-alpha): ...`)
- `alpha` to `beta` is keyed off of branch creation
- `beta` to `stable` is keyed of of a PR with a new conventional commit type and scope (e.g. `release(20.04-stable): ...`)

### Assets

- Assets will continue to work as they do today

### Release Lead Responsibilities

- Verify that the generated CHANGELOG looks good.
- Verify that any breaking changes are communicated.
- Verify that the set of assets look correct.
- Publish the GitHub release (i.e. move it out of draft).

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Compatibility

[A discussion of the change with regard to the change]

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not know the solution.][this section may be omitted if there are none.]
