# Conformance benchmark

Experimental draft PR; do not merge runner sizing without comparing CI results.

The parallel baseline uses eight vCPUs, seven Ginkgo processes and seed 1729.
The serial job keeps its existing selection and runs without parallel workers.
All experiments must compare the same upstream base, conformance image digest,
suite selection, and seed. Run additional seeds before making a production choice.

Successful and failed jobs retain the e2e results, including Ginkgo's JSON report.
Sonobuoy's mounted results directory is `/tmp/sonobuoy/results`; JSON reports
must be written there, not to the upstream image's unmounted `/tmp/results`.
The host resource log samples CPU, memory and I/O pressure every ten seconds.
GitHub step timings separate artifact transfer and bootstrap from execution;
the explicit start/collection timestamps separate Sonobuoy from Ginkgo runtime.
Compare selected spec identities, runtime skips, failures, per-worker duration,
total job duration and runner cost rather than relying on elapsed time alone.

The v1.37.2 image is digest-pinned across experiments. The local dry-run inventory
contains 462 conformance specs. The previous serial regex selected zero because
some names put `[Conformance]` before `[Serial]`. Both orders are now supported,
and the serial lane has an explicitly empty skip filter. Each CI job audits
the complete coverage and absence of overlap of the parallel and serial dry-run inventories.
Coverage restoration means the serial lane is not comparable with the previous
empty serial jobs; compare benchmark branches with each other instead.

The launcher uses one bounded Sonobuoy wait instead of trying to wait on a
parent-shell PID from a new shell. A launcher failure or missing/failed results
must fail the job. No kubeconfig or node credentials are uploaded as benchmark
artifacts.

## Direct upstream experiment

The `conformance-bench-direct-8cpu-7workers` branch selects
`CONFORMANCE_RUNNER=direct` only in the PR benchmark step. The Docker Makefile
wrapper forwards this setting; unset it (or use `sonobuoy`) for the existing
Sonobuoy launcher. Other test workflows retain Sonobuoy by default.

Direct mode uses the same digest-pinned `registry.k8s.io/conformance:v1.37.2`
image, shared suite arrays and one-server/one-agent Docker cluster. A Linux pod
runs the image's bundled `kubeconformance` Go entrypoint with an in-cluster
service-account identity with Sonobuoy's e2e RBAC rules (all API resource
operations, plus GET on `/metrics`, `/logs` and `/logs/*`).
Networking, mounts, image pull policy and tolerations follow Sonobuoy's Job defaults: no
host networking, host mounts or privileged container. A non-root second container
using the same image keeps the shared result volume available after e2e exits,
with a read-only result mount and root filesystem and dropped capabilities.

Image configuration was inspected with `crane config`. Its `E2E_PARALLEL=1`
means **no automatic parallelism**, not one selected worker: only `y`, `Y` or
`true` requests Ginkgo's automatic `--p`. Direct mode keeps `1` and passes
`--procs=7 --seed=1729` through `E2E_EXTRA_GINKGO_ARGS` for parallel tests.
Serial tests have no `--procs` or `--p`, using Ginkgo's single-process default
and the corrected both-tag-order focus with an empty skip. The existing
dry-run inventory audit uses these same selectors. No e2e binary flags are
invented or bypassed.

Ginkgo receives `--timeout=30m`; the pod completion wait is independently
bounded to 30 minutes, including scheduling/image startup. Timeout signals
the Go runner with INT, allows at most 60 seconds for report finalization,
then collects results while preserving the timeout exit code. Any nonzero
e2e exit, launch error, missing JSON or retrieval error fails the job.
On success and failure, artifacts retain the full result volume as
`e2e-results.tar.gz`, plus launcher output, JSON, manifest and exit code.
The archive is never extracted on the host; service-account projections and
host kubeconfigs are outside the archived volume. The temporary namespace and
RBAC binding are removed after collection. The unchanged host monitor and
artifact upload cover both launchers.

Validate locally with `bash tests/docker/conformance-direct-test` and
`bash tests/docker/sonobuoy-helpers-test`. Inventory and entrypoint dry-runs
can run in disposable Docker containers without accessing an existing cluster.
Ginkgo only supports dry-run in serial mode; the seven-worker entrypoint smoke
uses an intentionally empty selection and disabled Docker networking, not a
full conformance run.
Full in-cluster scheduling, API access and conformance behavior still require
the PR benchmark runs; do not infer correctness or speed from dry-run timings.
