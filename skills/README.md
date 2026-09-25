# Skills

Agent Skills for working with Beacon from a coding agent such as Claude Code. Each
directory holds one skill: a `SKILL.md` whose frontmatter description is what the agent
selects on, and the instructions it follows once selected.

| Skill | What it does |
|---|---|
| [`beacon-memory-curate`](beacon-memory-curate/SKILL.md) | Reads the sessions Beacon recorded, decides which hold a reusable lesson, and writes, approves, rejects or supersedes memory candidates with `beacon memory`. |

## Installing locally

Claude Code loads skills from `~/.claude/skills/<name>/SKILL.md`. Symlink a directory
from this tree so edits here are picked up without copying:

```bash
ln -s "$(pwd)/skills/beacon-memory-curate" ~/.claude/skills/beacon-memory-curate
```

The skills call the `beacon` CLI on `PATH`, so they need a release that carries the
commands they name. `beacon-memory-curate` needs `beacon memory candidates create`.
