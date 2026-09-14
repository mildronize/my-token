# Ticket 2 Report

## Ticket
Backend: additive `source_type` field on `GET /api/bff/usage/scan-roots`.

## Outcome
done

## Notes

- Clean, small, additive — no ambiguity worth a Decision section.
- Verified before building that ticket 1's work was NOT yet on
  `origin/story-2/integration` (checked directly rather than assuming),
  so branching fresh off `story-2/integration` was safe and didn't
  silently inherit ticket 1's diff.
- Branch pushed, no PR opened (per มายด์'s "one PR per story" course
  correction mid-loop): `story-3/ticket-2-scan-roots-source-type`.
- `seedScanRoot` test helper's `source_type` was hardcoded before this
  ticket; changed to a parameter so the new assertion is meaningful
  (exercises a non-default value, can't pass by coincidence). All 4 call
  sites updated.
- `go build`/`go vet`/`go test ./...` green; `gofmt -l` clean except one
  pre-existing, unrelated issue in `internal/collector/path_test.go`
  (confirmed via `git stash` to predate this ticket).
- `/chief-review-code`: Spec clean. Standards: one smell noted (same
  rationale comment repeated near-verbatim across 5 layers) but judged
  consistent with this repo's own existing per-layer-comment convention,
  left as-is rather than extracted.
