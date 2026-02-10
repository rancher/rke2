# About RKE2 docs

The RKE2 user docs are hosted on the [rancher/rke2-docs](https://github.com/rancher/rke2-docs) repository.

This RKE2 git repository currently holds two kinds of documentation:

- RKE2 Developer Docs
- RKE2 Architectural Decision Records (ADRs)

There are other differences than their audiences, and this document elaborates on it.

## User docs

As its name suggests, the main target for this documentation is Rancher RKE2 users. Covering RKE2 topics such as architecture, installation and upgrade process, and security from an end-user perspective.

Check the [rancher/rke2-docs](https://github.com/rancher/rke2-docs) repository for more information.

## Developer docs

Like this file, the target audience for these documents is the RKE2 developers and contributors. The topics may not hold a specific order, technicalities may come along, and are focused on transmitting internal processes around RKE2.

The developer docs are the markdown files in this repository's `developer-docs/` directory. These files are intended to be read using any markdown preview tool, being Github's web view the default one, so no enhanced versions of this markup language are allowed. The only exception to this rule is usage of [embedded mermaid diagrams supported by Github](https://github.blog/2022-02-14-include-diagrams-markdown-files-mermaid/).

As hinted in the last section, the diagrams within the developer docs are written in the markdown-like [Mermaid](https://mermaidjs.github.io/) syntax and held in code blocks with the <code>```mermaid</code> language specifier. These diagrams can be created and edited with the help of the [Mermaid live editor](https://mermaid.live/); then, it is a matter of copying and pasting the result in a markdown file.

## Architectural Decision Records (ADRs)

ADRs are a record of the arguments and decisions made to change process or software architecture. The idea is for these records to be regularly reviewed, updated, and documented so that new people or external parties to a project can read along and understand the points of a decision, the context, and in general can make educated decisions based on previous discussions.

ADRs provide a safer place for individuals who would rather not speak up in face to face communication (which can feel confrontational) or would like to educate themselves on a topic before commenting.

See the [docs/adrs](../docs/adrs) directory for more information.

