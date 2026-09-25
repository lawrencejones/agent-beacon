---
name: beacon-memory-curate
description: >
  Curate Beacon's cross-harness memory: read the agent sessions Beacon has recorded,
  decide which hold a lesson worth keeping, and write, approve, reject or supersede
  memory candidates so future agents in that repository are served something true and
  useful. Use whenever you're working with Beacon memory in any way — the dashboard's
  Memory page looks empty, `beacon memory` candidates are score placeholders, someone
  asks to review or curate what agents learned, or a scheduled sweep of new sessions.
  Not for Beacon capture problems (events missing from the runtime log) — that is
  `beacon endpoint doctor`'s job.
---

# Curate Beacon memory

Beacon records every agent session into a local runtime log on its own. Nothing after
that is automatic: `memory.db` only changes when someone runs a `beacon memory`
command, and the Jev evaluator that `evaluations run` calls returns three probabilities
without ever reading the trace, so every candidate it produces says "no lesson text was
extracted". **You are the reviewer this loop was designed around.** Whatever you approve
is served verbatim to every future agent working in that repository through the
`search_memory` and `get_memory_context` MCP tools, so a vague or wrong memory is worse
than none.

This skill covers turning recorded sessions into approved memory. Why events are
missing from the log is `beacon endpoint doctor` and the inventory docs; tuning Jev is
the `beacon memory evaluations` docs; installing approved memory as an Agent Skill is
`beacon memory skills`, which you may point the user at but do not run unasked.

## Vocabulary

- A **trace** is one recorded unit. Session traces are `session:<harness>:<id>` and are
  the only kind worth reading. `event:<id>` traces are single events with no session,
  almost always OTLP metric samples such as `claude_code.active_time.total`; skip them.
- A **candidate** is a proposed memory in state `candidate`, `approved`, `rejected` or
  `superseded`. One with a `source_evaluation_id` came from Jev and carries the
  placeholder body. One without was written by a reviewer with `candidates create`.
- A **memory** is what approval produces. `beacon memory list` shows only memories that are
  not superseded, which is also what the MCP tools serve.
- **Kinds**: `workflow` (a sequence that works here), `correction` (something an agent
  did that had to be undone), `debugging_pattern` (symptom to cause to fix),
  `gotcha` (a trap in this repo or its tooling), `convention` (how this project does
  things). Pick the one a future agent would search for.
- **Events have a coarse `type` and a fine `action`.** `--event-type` on `traces list`
  and `traces show` filters `type`: `user_message`, `tool_call`, `command`,
  `tool_result`, `approval`, `error`, `file`, `mcp`, `session`, `token_usage`, `metric`.
  Names like `prompt.submitted` or `command.executed` are `action` values and match
  nothing in that filter.
- **The log usually holds no assistant text.** For Claude Code, a session's substance
  is its `user_message` events (the only ones with a `content` object), its `tool_call`
  and `command` events (tool name and command line), and whether a `tool_result` was
  a failure. Tool output and the agent's own words are not recorded. A lesson has to be
  evidenced from what the user asked, what commands ran, which failed and what ran
  next.
- **Project** is the repository a trace recorded, resolved to its git root and remote,
  so a linked worktree resolves to the same project as its main checkout. Commands
  that start from a trace scope to that repository. Commands that list
  (`candidates list`, `beacon memory list`) scope to the current directory unless you pass
  `--project <path>`, so a listing from the wrong directory reads as empty when it is
  not.
- **`supersede` takes the old candidate's ID and the new memory's ID.** Memories carry
  their `candidate_id`; read it from `beacon memory show --json` rather than guessing.

## Where things are

| Need | Where |
|---|---|
| Recent sessions, newest first | `beacon endpoint traces list --json --limit <n> --page <p>`: `id`, `title`, `updated_at`, `event_count`, `repository.path`, `token_usage` |
| One session's events | `beacon endpoint traces show <trace-id> --json --event-type user_message,tool_call,command,tool_result,approval,error --limit <n> --offset <k> > <file>`: bounded window; `range.total_events` counts the filtered events, so it says how much you have not read |
| Pending Jev placeholders | `beacon memory candidates list --state candidate --json --project <path>` |
| Everything already reviewed for a repo | `beacon memory candidates list --json --project <path>`: every state; `evidence[].trace_id` is the ledger of traces already turned into candidates |
| What agents are served today | `beacon memory list --json --project <path>`, then `beacon memory show <memory-id>` |
| Record a lesson from a trace you read | `beacon memory candidates create --trace <id> --kind <kind> --title <t> --body-file - [--applicability <a>] [--tag <t>]...` |
| Approve, optionally fixing the text | `beacon memory candidates approve <candidate-id> --reason <why> [--kind] [--title] [--body-file -] [--applicability]` |
| Decline a candidate | `beacon memory candidates reject <candidate-id> --reason <why>` |
| Replace an older memory | `beacon memory candidates supersede <old-candidate-id> --replacement <new-memory-id> --reason <why>` |

Write multi-line bodies to a file and pass `--body-file <path>`, or pipe them through
`--body-file -`, rather than quoting them on the command line. `beacon memory list --json`
prints `[]` when empty, but `candidates list --json` prints `null`; handle both. Where
the session also has Beacon's MCP tools, `search_activity` and `get_activity_event`
read the same log and `search_memory` reads the same store, but the write path is the
CLI only.

`traces show` on a large log takes tens of seconds unfiltered and can exceed a tool
timeout; the event-type filter above brings it to seconds. Always redirect the JSON to a
file and read the file, so a slow call is never lost to a parser that fails first.

If `beacon memory candidates create --help` fails, the installed Beacon predates
authored candidates. Say so, list the lessons you would have written in the reply, and
stop: approving a placeholder body is not a fallback.

## Reading results

- **A `traces list` with no `session:` IDs is not "no sessions".** Metric samples arrive
  every few seconds and each becomes an `event:` trace, so a small `--limit` can be all
  noise. Page further or raise the limit until you have sessions, then stop.
- **`event_count` is not substance.** Hooks and OTLP both record, so a session that did
  one thing shows hundreds of `token_usage` and `session` events. Judge a session by
  its `user_message`, `tool_call`, `command` and `tool_result` events, and skip
  sessions whose only prompt is a task notification, a slash command or a one-line
  question.
- **A session still being written is not ready.** `updated_at` within the last ten
  minutes, or `event_count` that differs from `range.total_events` between two reads,
  means the agent is still working. Leave it for the next sweep; a lesson written from
  half a session is usually wrong about how it ended.
- **`repository: null` means the session did not record where it ran.** Roughly half
  of Claude Code sessions look like this. In a sweep scoped to one repository, drop
  them and count them in `Skipped`. In an unscoped sweep, read one only if its title
  is promising, and write from it only with an explicit `--project <path>` you can
  justify from paths in its `command` events; otherwise skip it.
- **Content may be absent by policy.** A `user_message` with `content.included: false`
  or a bare hash means Beacon retained metadata only. Other event types never carry a
  `content` object, which is normal. You cannot extract a lesson from a hash; say the
  trace was unreadable rather than inferring from event names.
- **An empty `candidates list` is empty for that project path.** Before concluding
  nothing was reviewed, check `--project` matches the trace's `repository.path`.
  Candidate JSON carries every event ID of its trace as evidence, so a listing can run
  to tens of kilobytes; read `evidence[].trace_id` from it rather than the whole thing.
- **Your own sweep is in the log.** The session running this skill appears in
  `traces list` with this skill's commands in it. Never write a memory from it.

## The sweep

The job needs a repository and a window. Take the repository from the request or from
the sessions themselves (group by `repository.path`); when the request names none, sweep
every repository that has sessions in the window and say so. Default the window to
sessions with `updated_at` after the newest candidate `created_at` for that project,
and to the last 7 days when the project has none. Never guess a window silently: state
it in the reply.

1. **Clear the pending queue.** For each `candidates list --state candidate` entry, read
   its `evidence[0].trace_id` as in step 3, then either `approve` with `--kind`,
   `--title` and `--body-file -` carrying the real lesson, or `reject --reason`. A Jev
   placeholder is never approved as it stands. Done when the state filter returns
   nothing.
2. **Select sessions.** List traces with a limit large enough to get past the metric
   noise, then apply the filters in this order: keep `session:` IDs inside the window;
   drop the current session and any still being written; drop those whose ID already
   appears in a candidate's `evidence[].trace_id`; drop one-line and slash-command
   sessions by title; drop `repository: null` sessions in a scoped sweep. Cap what is
   left at 15 per run, newest first. Done when you have the list and have written down
   what you dropped and why.
3. **Read each session bounded.** Fetch the first 150 filtered events to a file and,
   when `range.total_events` is larger, the last 150 with `--offset`. Read the middle
   only when the ends show a correction in progress. Done when you can say in one
   sentence what the user asked, which commands failed or were redone, and what the
   user asked next.
4. **Decide.** A session earns a memory only when all four hold: a future agent in this
   repository would act differently for knowing it; the trace itself shows the evidence
   (a failed command and what ran instead, a user correction and what followed, a
   convention stated and applied); it is not already in `beacon memory list` or in the
   repository's own docs; and it contains nothing that must not be served (tokens,
   customer names, paths under a home directory that identify a person). Because the
   log holds commands but not their output, the strongest evidence is a `tool_result`
   failure followed by a different command that succeeded. Most sessions earn nothing.
   That is the expected result, not a failure.
5. **Write.** `candidates create` then `approve --reason` in the same turn. Title in the
   imperative under 80 characters. Body of three to eight lines: the lesson, why it holds
   here, how to apply it, with identifiers copied exactly from the trace, and one line
   naming what would retire the memory (a doc fix, a renamed flag). Applicability as the
   moment a future agent should reach for it. When the lesson refines an existing
   memory, create and approve the new one, then `supersede` the old candidate with the
   new memory ID. Done when `beacon memory show` prints the text you meant.

When the request says "show me first" or "propose", run steps 1 to 5 but stop before
`approve`: create the candidates, and put their IDs and text in the reply for a person
to approve.

A well-formed body reads like this, and nothing like a session narrative:

```text
`ginkgo -r` from the server root compiles every package and takes minutes; CI already
does that sweep. Run `ginkgo ./path/to/changed/package/` for the packages the change
touched. A build wider than the test scope is still worth keeping, because it catches
breakage in sibling packages.
```

## Output

End every sweep with this block in the reply itself, whatever else it says. Whoever
asked is reading in chat and cannot open `memory.db`.

- **Scope:** repositories and window, and whether the window was given or defaulted
- **Approved:** one line each: memory ID, kind, title, source trace ID
- **Rejected:** one line each: candidate or trace, the reason recorded
- **Superseded:** old memory → new memory, one line each
- **Skipped:** sessions read and left alone, with the one-sentence reason; sessions not
  read because of the cap or unreadable content, marked as such
- **Next:** the one thing a person should do: run `beacon memory skills install` for a
  memory that belongs in the repo, widen the window, or nothing

Every heading survives an empty run: `Approved: none` is a finding. A repository whose
traces could not be read is `Skipped`, never silence.

## Cost and privacy

Reading a session sends its prompts, commands and assistant text to the model you are
running on. Beacon's own hooks never do this; you do it on the user's behalf, so run
only where that is acceptable, and never paste trace content into the reply beyond the
identifiers the output block needs. Fifteen sessions at 300 events each is the budget
for one run. After two sessions in a row yield nothing readable, report what you have
and name the gap rather than paging on.

## What this skill doesn't do

- **Score traces with Jev.** `beacon memory evaluations run` is a cheaper pre-filter
  that you may use to rank sessions when there are far more than the cap, but its
  candidates still need step 1.
- **Install memory as an Agent Skill in the repository.** Suggest it in `Next` and let
  the user run `beacon memory skills install`; it writes into their checkout.
- **Fix missing capture.** No sessions at all for a repository that has had agent work
  means Beacon is not recording there. Point at `beacon endpoint doctor` and stop.
