# Local check before every push

Run this from the repository root in PowerShell before every push ([ADR-0033](../decisions/ADR-0033-stage-1-trust-and-measurement.md) D13). Go is not on this machine's path, so it runs in the `golang:1.26` image. It does what CI's lint and test steps do, in this order, and stops at the first failure:

1. **`gofmt -l .`** lists no file.
2. **`templ generate`** runs at the version in `go.mod`, and no `*_templ.go` file changes. A change means a generated file was edited by hand or not regenerated. The check leaves the regenerated files in place; review and commit them.
3. **`go vet ./...`** passes.
4. **`go test -count=1`** passes on the packages in `$env:PKGS`, or on every package when it is unset.

```powershell
docker run --rm -v "${PWD}:/src" -v hebrew-gomod:/go/pkg/mod -v hebrew-gocache:/root/.cache/go-build -w /src -e PKGS golang:1.26 bash -c 'set -eu; echo [1/4] gofmt; gofmt -l . | tee /tmp/fmt; test ! -s /tmp/fmt; echo [2/4] templ generate drift; find . -name \*_templ.go -not -path ./node_modules/\* | sort | xargs md5sum > /tmp/before; GOBIN=/tmp/bin go install github.com/a-h/templ/cmd/templ@$(go list -m -f {{.Version}} github.com/a-h/templ); /tmp/bin/templ generate; find . -name \*_templ.go -not -path ./node_modules/\* | sort | xargs md5sum | diff /tmp/before -; echo [3/4] go vet; go vet ./...; echo [4/4] go test; go test -count=1 -timeout 20m ${PKGS:-./...}; echo LOCAL CHECK PASSED'
```

It passed when it printed `LOCAL CHECK PASSED`. To test only some packages, set them first, separated by spaces:

```powershell
$env:PKGS = './internal/web/... ./internal/db/...'
```

Clear the variable with `Remove-Item Env:PKGS` to go back to every package.

- **Volumes.** The two named volumes keep the module and build caches between runs, so only the first run downloads modules.
- **What CI adds.** CI also runs the race detector (`go test -race`) and the node and Chrome tests in `internal/web`. Those tests skip in this image.
