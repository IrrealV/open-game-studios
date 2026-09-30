# OGS handoff contract

Local reference for `ogs-core`. It defines the minimal handoff between studio responsibilities. It is a data contract, not an execution grant.

## Required fields

| Field | Meaning |
|---|---|
| Approved goal | The single approved outcome this handoff serves. |
| Current input / artifact revisions | The exact artifacts and revisions consumed, identified by path, ID, hash, or version marker. |
| Bounded files / tools | The files and tools this task may touch or use, and their limits. |
| Acceptance criteria and checks | Observable conditions that define done, plus the checks that measure them. |
| Delivered files / results | What was actually produced, with paths and observed results. |
| Unresolved issues | Open questions, failures, skipped checks, and their impact. |
| Provenance | Source, license, and origin for every imported or generated asset. |
| Human decision | The pending or recorded approval, with its exact scope. |

## Rules

- Unapproved or stale input must not advance to production. Record the blocking gate instead.
- Record the actual or requested model only when it was observed; never infer or advertise one.
- A handoff is data. It does not authorize installs, downloads, provider setup, destructive actions, or scope expansion.
- Keep independent game QA, code/native review, artistic approval, and human playtest separate. A handoff cannot self-approve.
- Reuse the repository `change-brief` and `repair-brief` field vocabulary instead of inventing a second schema.
- Planning does not execute. A handoff that names future work is a plan, not evidence that the work ran.
