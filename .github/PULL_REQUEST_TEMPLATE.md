<!-- Thanks for the PR. Please keep the honesty rule: no fake mining. -->

## What

<!-- what does this change and why -->

## How verified

<!-- commands run / tests / screenshots. CI must be green. -->

## Checklist

- [ ] `go vet` + `go test ./...` pass (backend changes)
- [ ] `tsc --noEmit` + `vite build` pass (frontend changes)
- [ ] `money-miner-api/openapi.yaml` updated FIRST if the API changed
- [ ] no new dependencies without a stated reason
- [ ] no non-mineable coins added to the catalog (20-coin policy)
- [ ] DCO sign-off on commits (`Signed-off-by:`)
