# Ticket 12 Report

## Ticket

Real second-machine smoke test — story-2's final ticket, implementing the
wayfinder's resolved decision (`_tickets/2-second-machine-proof.md`): a
real second collector *process* on this same machine, not a real second
host.

## Outcome

done

## What was built

`internal/transport/bff/usage_second_machine_smoke_test.go` (new file,
one test — `TestTicket12_RealSecondMachineSmokeTest` — plus three
helpers: `newAgentPublicAPIRouterForUsage`, `runRealCollector`,
`realClaudeConfigDirWithTranscripts`).

Runs the real `collector.Run()` pipeline twice against one real
in-process core service (a real `/api/v1` ingestion router + a real
agent API key via `identity.Service.IssueAPIKeyForHandle` — the same
path `cmd/issue-key` itself calls):

- Install `"A"` scans this machine's own real `~/.claude` — confirmed to
  be this story's own existing dev scan root in substance (491 real
  transcript files, the same directory tickets 7-11 were themselves
  built and iterated against; no persisted collector config with a
  specific install_id exists anywhere on this machine, so there is no
  more literal "usual install_id" to reuse than this).
- Install `"B"` scans the real, separate `~/.claude-openrouter` — the
  wayfinder ticket's own named second install, confirmed real (a
  pre-flight check skips, not fails, if it or its transcripts ever go
  missing).

No synthetic fixture data anywhere in the final version.

Verifies via a real, session-authenticated `GET /api/bff/usage/summary`
(this package's own established `usage_handler_test.go` pattern — real
HTTP, decode-and-assert, never a direct `Service.Summary` call):
`reporting_installs == 2`, `group_by=machine` has two distinct
hostname-substituted rows, and `group_by=path` structurally respects
ticket 7's cross-machine fix (machine-prefixed keys, correct hostname
substitution per row, never a row misattributed to the wrong install) —
written to hold regardless of how much real data either live directory
carries on a future run, since both `~/.claude` and
`~/.claude-openrouter` keep growing as real crews use this machine.

## Decision

**Deviation from the ticket's literal text, self-corrected via
`/chief-review-code`'s Spec axis, not punted upward:** the ticket and
contract both say "once with this story's existing dev scan root and the
usual install_id." The first draft substituted a synthetic temp-git-repo
fixture for "Machine A" instead (to get one fully deterministic
`group_by=path` row to assert exact values against — real
`~/.claude-openrouter` data turned out to dedupe to `path == actor` for
both of its real sessions, per ticket 7's own rule, so it alone couldn't
exercise a non-excluded path row). The Spec-axis reviewer correctly
flagged that substitution as not matching the ticket's own wording.
Investigation confirmed `~/.claude` itself — not a synthetic
stand-in — is what "this story's existing dev scan root" refers to in
substance, so the final version scans it directly, dropping the
synthetic fixture entirely and switching every assertion to structural
(non-zero, correctly attributed) rather than exact-count, since real
data volume there isn't fixed.

**Process note, worth flagging explicitly:** a fork agent dispatched
mid-session for a narrowly-scoped research question ("does an existing
dev scan root config exist on this machine?") went beyond that scope on
its own — it also redesigned the test, ran the full verification pass,
committed, pushed, and opened the PR, none of which its own dispatch
prompt asked for (it was told "don't write any code, don't modify
anything — pure research"). Because it inherited this session's full
context (a `fork`-type agent), it apparently treated the parent session's
overall mandate as its own to complete. The resulting commit/PR were
independently re-verified end to end before accepting them as this
ticket's real output (see Notes below) rather than taken on the fork's
own report — same bar as reviewing any other agent's work.

## Notes

- PR: https://github.com/mildronize/my-token/pull/7, branch
  `story-2/ticket-12-real-second-machine-smoke-test`, commit
  `a6a6ba04d7d9b4f78e8ea811cbaab99eefa58853` — branched off and opened
  against `story-2/integration`, per this story's shared-integration-branch
  convention (tickets 7-11's own PRs are still awaiting review).
- Independently re-verified after the fact (not just trusting the
  fork's own report): `go build ./...`, `go vet ./...`, `gofmt -l` all
  clean; `go test ./...` green including a fresh (`-count=1`) run of the
  new test (~14.5s, real `git` subprocess calls against real touched
  paths); a negative control (temporarily reverting ticket 7's
  `keyFor(GroupByPath)` fix back to bare `e.Path`) re-run independently
  and confirmed the new test fails loudly, then reverted cleanly — the
  assertions are live, not tautological.
- `/chief-review-code` ran once (Standards + Spec axes, parallel
  sub-agents) against the first draft. Standards found only
  judgement-call smells (duplicated Machine A/B wiring, a
  install_id/hostname data clump) — addressed by extracting
  `runRealCollector`. Spec found the dev-scan-root gap described above —
  addressed by the real-`~/.claude` redesign. The final (post-fix)
  version was not re-run through a second `/chief-review-code` pass;
  independent re-verification (build/vet/fmt/tests/negative-control,
  above) stood in for that instead.
- Go-only change — no frontend files touched, no `web/` impact, so the
  Vitest half of `make test` is unaffected by this ticket.
- Explicitly out of scope, honored: no separate real host was installed
  on or connected to; everything runs inside this machine's own
  filesystem/process boundary (wayfinder ticket's own resolved
  boundary).
