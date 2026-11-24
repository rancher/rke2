# Conformance benchmark

Experimental draft PR; do not merge runner sizing without comparing CI results.

The parallel baseline uses eight vCPUs, seven Ginkgo processes and seed 1729.
The serial job keeps its existing selection and runs without parallel workers.
All experiments must compare the same upstream base, conformance image digest,
suite selection, and seed. Run additional seeds before making a production choice.

Successful and failed jobs retain the e2e results, including Ginkgo's JSON report.
The host resource log samples CPU, memory and I/O pressure every ten seconds.
GitHub step timings separate artifact transfer and bootstrap from execution;
the explicit start/collection timestamps separate Sonobuoy from Ginkgo runtime.
Compare selected spec identities, runtime skips, failures, per-worker duration,
total job duration and runner cost rather than relying on elapsed time alone.

Serial jobs currently report zero specs. Audit this against the exact image's
inventory before changing the filters or removing the serial lane. Empty serial
runs are not evidence that full intended conformance coverage was exercised.

The launcher uses one bounded Sonobuoy wait instead of trying to wait on a
parent-shell PID from a new shell. A launcher failure or missing/failed results
must fail the job. No kubeconfig or node credentials are uploaded as benchmark
artifacts.
