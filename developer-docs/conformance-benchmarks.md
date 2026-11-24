# Conformance benchmark

Experimental draft PR; do not merge runner sizing without comparing CI results.

This experiment uses sixteen vCPUs, ten Ginkgo processes and seed 1729.
The serial job uses eight vCPUs and runs without parallel workers.
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
complete coverage and absence of overlap in the parallel and serial inventories.
Coverage restoration means the serial lane is not comparable with the previous
empty serial jobs; compare benchmark branches with each other instead.

The launcher uses one bounded Sonobuoy wait instead of trying to wait on a
parent-shell PID from a new shell. A launcher failure or missing/failed results
must fail the job. No kubeconfig or node credentials are uploaded as benchmark
artifacts.
