# Agents

## Spec-driven development (OpenSpec)

This project uses OpenSpec. Specs are the source of truth and code follows them.

- OpenSpec root: `openspec/` (`openspec/specs/` for current specs, `openspec/changes/` for active changes).
- Non-trivial work starts with a change proposal, not with code edits. Use the OpenSpec commands:
  - `/opsx-propose "idea"` — create a full change (proposal, design, specs, tasks).
  - `/opsx-update` — revise planning artifacts of an active change.
  - `/opsx-apply` — implement tasks from an active change.
  - `/opsx-sync` — push delta specs into main specs without archiving.
  - `/opsx-archive` — archive a completed change and update main specs.
  - `/opsx-explore` — think through a problem before proposing a change.
- Useful CLI commands: `openspec list`, `openspec show <item>`, `openspec validate`, `openspec status`, `openspec view`.
- Write all OpenSpec artifacts in English, keeping structural headings and SHALL/MUST keywords in English.
- Keep changes small and focused; one change per logical unit of work.
