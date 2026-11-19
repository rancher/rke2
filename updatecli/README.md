# Updatecli

[Updatecli](https://www.updatecli.io/) keeps the charts and images bundled with
RKE2 up to date. The manifests in `updatecli.d` are applied daily by the
`.github/workflows/updatecli.yml` workflow on `master` and the `release-1.*`
branches, using the values in `values.yaml`.

To test locally, set `UPDATECLI_GITHUB_ACTOR` to your GitHub username and
`UPDATECLI_GITHUB_TOKEN` to a GitHub token, then run:

```sh
updatecli pipeline diff --config ./updatecli/updatecli.d/ --values ./updatecli/values.yaml
```

Never use the `--debug` option, because it might leak the access tokens. The
CNI manifests run `scripts/create-issue.sh`, which can create or edit issues in
`rancher/rke2` even during a `diff`.

## Pull request grouping

Manifests that share a top-level `pipelineid` share the working branch
`updatecli_<branch>_<pipelineid>`, so each group opens or updates a single pull
request per target branch:

| `pipelineid` | Manifests | Pull request title |
| ------------ | --------- | ------------------ |
| `rke2-cni` | `calico-cni`, `canal-cni`, `cilium-cni`, `flannel-cni`, `multus-cni` | `[<branch>] Update CNI charts` |
| `rke2-hardened-runtime` | `hardened-runtime-images` | `[<branch>] Update hardened runtime images` |
| `rke2-charts` | `coredns`, `ingress-nginx-prime`, `klipper-helm`, `klipper-lb`, `metrics-server`, `traefik`, `vsphere-cpi`, `vsphere-csi` | `[<branch>] Update bundled charts and images` |

When adding a new manifest to `updatecli.d`:

* Set the `pipelineid` of the group it belongs to.
* Use the same `github/pullrequest` action settings as the other manifests in
  the group, including the exact same `title`. Updatecli rewrites the title of
  an existing pull request on every pipeline run, so a different title would
  make the pull request title flap.
* Give the pipeline a unique `name`. Updatecli merges the pull request
  description per pipeline, keyed by the pipeline name.
* Give each target a short `name` that includes the new version, e.g.
  `'Bump Foo chart to {{ source "foo" }}'`. Updatecli uses `chore: ` followed
  by the target name as the commit title (`commitmessage.title` is deprecated
  and ignored), so each bump stays a distinct, identifiable commit in the
  grouped pull request. Keep the rendered name under 65 characters.
* Don't set `commitmessage.squash: true`, because it would squash every bump
  in the group into one commit.

Trade-off: a single failing bump blocks the whole grouped pull request. To ship
the remaining bumps, revert the offending commit on the pull request branch (or
fix it there) before merging.
