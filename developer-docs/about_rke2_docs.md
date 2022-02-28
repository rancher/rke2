# About RKE2 docs

The RKE2 git repository currently holds two kinds of documentation:

- RKE2 User Docs
- RKE2 Developer Docs

There are other differences than their audiences, and this document elaborates on it.

## User docs

As its name suggests, the main target for this documentation is Rancher RKE2 users. Covering RKE2 topics such as architecture, installation and upgrade process, and security from an end-user perspective.

The markdown files in the `docs/` directory within this repository are the documentation's source. These documents are processed using [mkdocs](https://www.mkdocs.org/) and served on <https://docs.rke2.io/> as a documentation website.

Since the documents use a specific markdown superset, [pymdown](https://facelessuser.github.io/pymdown-extensions/), it is preferred to process them using `mkdocs` beforehand to have a better understanding. However, any markdown preview tool should be able to render them.

To serve the RKE2 user docs website locally, one can either:

1. Run `make serve-docs` [Makefile](../Makefile) target. This will start a docker container locally with all the required configurations and serve them on port `8000`.
2. Run `mkdocs serve` in the local environment. This will start the mkdocs server locally, exposing port `8000`. Since the said tool is written in [python3](https://www.python.org/), it requires installing its interpreter and the following packages beforehand `mkdocs`, `mkdocs-material`,  `mkdocs-markdownextradata-plugin`, and `pymdown-extensions`.

Worth noting that the second option should only be used whenever running a docker container locally does not work correctly (i.e., working on non-Linux OS's or needing to deal with shared mounts).

## Developer docs

Like this file, the target audience for these documents is the RKE2 developers and contributors. The topics may not hold a specific order, technicalities may come along, and are focused on transmitting internal processes around RKE2.

The developer docs are the markdown files in this repository's `developer-docs/` directory. These files are intended to be read using any markdown preview tool, being Github's web view the default one, so no enhanced versions of this markup language are allowed. The only exception to this rule is usage of [embedded mermaid diagrams supported by Github](https://github.blog/2022-02-14-include-diagrams-markdown-files-mermaid/).

As hinted in the last section, the diagrams within the developer docs are written in the markdown-like [Mermaid](https://mermaidjs.github.io/) syntax and held in code blocks with the <code>```mermaid</code> language specifier. These diagrams can be created and edited with the help of the [Mermaid live editor](https://mermaid.live/); then, it is a matter of copying and pasting the result in a markdown file.
