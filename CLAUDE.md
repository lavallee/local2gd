<!-- BEGIN CUB MANAGED SECTION v1 -->
<!-- sha256:527f2e81d5176126a34dff1a34096090d6a7f39ce7aef0608cab8085c8c370d6 -->
# Cub Task Workflow (Claude Code)

**Project:** `local2gd` | **Context:** @.cub/map.md | **Principles:** @.cub/constitution.md

## Quick Start

1. **Find work**: `cub task ready` or `cub task list --status open`
2. **Claim task**: `cub task claim <task-id>`
3. **Build/test**: See @.cub/agent.md for commands
4. **Complete**: `cub task close <task-id> --reason "what you did"`
5. **Log**: `cub log --notes="session summary"` (optional)

## Task Commands

- `cub task show <id>` - View task details
- `cub status` - Project status and progress

## Claude-Specific Tips

- **Plan mode**: Save complex plans to `plans/<name>/plan.md`
- **Skills**: Use `/commit`, `/review-pr`, and other skills as needed
- **@ References**: Use @.cub/map.md for codebase context, @.cub/constitution.md for principles

## When Stuck

If genuinely blocked (missing files, unclear requirements, external blocker):
```xml
<stuck>Clear description of the blocker</stuck>
```

See @.cub/agent.md for full workflow documentation.
<!-- END CUB MANAGED SECTION -->
