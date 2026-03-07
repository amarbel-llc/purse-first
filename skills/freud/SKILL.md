---
name: freud
description: >-
  Use when the user asks to "analyze transcripts", "review past sessions",
  "find agent friction", "identify workflow improvements", "what keeps going
  wrong", "therapy session", "session retrospective", or wants to audit Claude
  Code conversation history for recurring tool failures, wasted context, ignored
  instructions, or repeated mistakes. Also applies when the user says "freud".
---

# Transcript Analysis for Agent Workflow Friction

## Overview

Systematic technique for mining Claude Code conversation transcripts to find
recurring patterns where agents waste time, ignore instructions, or misuse
tools. Produces a quantified report and routed TODO items.

**Core principle:** Large transcripts correlate with struggle. Parallel
extraction beats serial reading. Fixes belong in the repo that owns the code.

## When to Use

- User wants to improve CLAUDE.md instructions based on real failures
- User wants to identify tool/MCP improvements from usage patterns
- User wants to find skill gaps or skill improvements
- User asks "what keeps going wrong" or "why does Claude keep doing X"
- Periodic maintenance: run quarterly or after a burst of frustrating sessions

## Phase 1: Discover Transcripts

Transcripts live at `~/.claude/projects/`. Each subdirectory is a path-encoded
project directory (hyphens replace slashes). Inside each are `.jsonl` files —
one per conversation session.

```bash
# Find the project directory for the current repo
# e.g. /home/user/repos/my-project → -home-user-repos-my-project
ls ~/.claude/projects/

# List transcripts sorted by size (largest = most friction)
ls -lS ~/.claude/projects/<project-dir>/*.jsonl
```

**Select the top ~20 transcripts by file size.** Larger files mean more
back-and-forth, which correlates with tool failures, repeated attempts, and
context waste.

Skip transcripts under 100KB — they're too short to contain meaningful friction
patterns.

## Phase 2: Group and Dispatch Parallel Agents

Group the selected transcripts into 4-5 batches of ~4-5 files each. Group by
subproject or theme when possible (e.g., all dodder transcripts together).

Launch one background agent per batch using the Task tool with
`subagent_type: "general-purpose"` and `run_in_background: true`.

Each agent gets this extraction prompt (adapt the file paths):

```
Analyze these Claude Code conversation transcripts for patterns where the agent
struggled, wasted context, or made repeated errors.

Files to analyze:
- <path1>
- <path2>
- ...

For EACH transcript, extract:

1. **Task**: What was the user trying to accomplish? (1 sentence)
2. **Tool failures**: Which tool calls failed and why? Include the exact tool
   name, parameters that caused failure, and error message.
3. **Repeated attempts**: Where did the agent try the same approach 2+ times?
   How many attempts before success or giving up?
4. **Instruction violations**: Did the agent ignore CLAUDE.md rules? Which ones?
5. **Context waste**: What burned tokens without progress? (e.g., reading files
   that weren't relevant, speculative evaluations, excessive output not
   truncated)
6. **Recovery pattern**: When something failed, what did the agent do? (retry
   same thing, try alternative, fall back to Bash, ask user, give up)

Format each transcript as:
### <filename>
- **Task**: ...
- **Findings**: (bullet list of issues with specifics)

After all transcripts, add:
### Cross-Transcript Patterns
List any issues that appeared in 2+ transcripts. These are the high-value fixes.
```

## Phase 3: Synthesize Findings

Once all agents complete, merge their results:

1. Collect all cross-transcript patterns from each agent
2. Deduplicate — same root cause appearing across agent batches counts once
3. Rank by frequency (number of sessions affected) and severity (estimated
   wasted tool calls per occurrence)

## Phase 4: Categorize by Fix Layer

Sort each finding into exactly one category:

| Category | Fix goes where | Examples |
|----------|---------------|----------|
| **CLAUDE.md addition** | The repo whose CLAUDE.md should have prevented it | "always use `just test*`", "never dereference X" |
| **Tool/MCP improvement** | The tool's repo as a TODO | False positive validation, missing redirects, unhelpful errors |
| **Skill improvement** | The skill's repo as a TODO | Missing stop conditions, wrong tool recommendations |

## Phase 5: Quantify Impact

For each pattern, estimate:

- **Sessions affected**: How many of the analyzed transcripts showed this?
- **Wasted tool calls per session**: How many extra calls before recovery?
- **Wasted context tokens**: Rough estimate of tokens burned (1 unnecessary
  file read ≈ 2-4K tokens, 1 failed build with full output ≈ 5-10K tokens)

Present as a ranked table, highest impact first.

## Phase 6: Deep-Dive (Top Patterns Only)

For the top 2-3 patterns by impact, launch a targeted agent to extract exact
details:

- The precise tool call JSON that failed
- The exact error message returned
- What the agent did next (recovery behavior)
- What the agent *should* have done

This provides the specificity needed to write good CLAUDE.md instructions or
file actionable tool improvement TODOs.

## Phase 7: Output

### Report

Write findings to `docs/plans/YYYY-MM-DD-transcript-analysis.md` with:

```markdown
# Transcript Analysis: YYYY-MM-DD

## Summary
- Transcripts analyzed: N
- Total patterns identified: N
- Top category: CLAUDE.md / Tool / Skill

## Patterns by Impact

| # | Pattern | Category | Sessions | Est. Wasted Calls | Est. Wasted Tokens |
|---|---------|----------|----------|--------------------|--------------------|
| 1 | ...     | ...      | ...      | ...                | ...                |

## Detailed Findings

### Pattern 1: <name>
**Category**: CLAUDE.md addition / Tool improvement / Skill improvement
**Affected repo**: <repo name>
**Sessions**: N of M analyzed
**Root cause**: ...
**Evidence**: (exact tool calls and errors from deep-dive)
**Recommended fix**: ...

### Pattern 2: ...
```

### TODO Routing

After the report, append TODO items to each affected repo's `TODO.md`:

- CLAUDE.md fixes → that repo's TODO.md under `## CLAUDE.md improvements`
- Tool fixes → that tool's repo TODO.md under the relevant section
- Skill fixes → the skill's repo TODO.md

**Routing rule**: Fixes go to the repo that owns the code being fixed, not
the repo where the friction was observed. A dodger build failure caused by
missing CLAUDE.md instruction → dodger/TODO.md, not eng/TODO.md.

Present the routed TODOs to the user for approval before writing them.

## Common Mistakes

- **Analyzing too few transcripts**: Need 15+ to spot patterns vs one-offs
- **Not using parallel agents**: Serial analysis of 20 transcripts burns the
  entire context window before synthesis
- **Fixing symptoms**: "Agent retried 3 times" is a symptom. "Tool validation
  rejects valid Go regex syntax" is a root cause
- **Routing fixes to the wrong repo**: The fix belongs where the code lives,
  not where the problem was observed
- **Skipping quantification**: Without impact numbers, all patterns look equally
  important. They're not

## Related Skills

- **bob:systematic-debugging** — For deep-diving individual high-impact patterns
- **bob:writing-plans** — If findings require multi-step implementation
- **bob:dispatching-parallel-agents** — Technique reference for the parallel
  agent dispatch in Phase 2
