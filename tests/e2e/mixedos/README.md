# How to switch CNI

Calico is the default CNI plugin for this E2E test. If you want to use Flannel instead, add "flannel" as the value for `E2E_CNI`

Example:

```
E2E_CNI=flannel go test -v -timeout=30m tests/e2e/mixedos/mixedos_test.go
```
