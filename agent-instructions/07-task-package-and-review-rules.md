# Task Package and Review Rules

## Goal

Task packages are implementation contracts, not only feature checklists. Future worktrees must make hidden constraints explicit before coding starts.

## Required task-package sections

Every new worktree task from P8 onward must include:

1. `必须保持的现有行为`
2. `允许的中间态`
3. `禁止的半迁移状态`
4. `失败模式与边界场景`
5. `必须新增或更新的回归测试`

These sections are required in addition to the normal fields: task name, goal, allowed files, forbidden files, dependencies, development content, security requirements, acceptance criteria, and test commands.

## Migration-task rules

For frontend or backend migration tasks, the package must state:

- The old production path that still exists.
- The intermediate state allowed after the task.
- The final target path owned by later tasks.
- Which old behaviors must remain functional until the replacement path is live.
- Which half-migrated states are forbidden.

If the replacement path is not live yet, a task must not silently break or misrepresent the old path. A prepared backend path may exist beside the old path, but the user-visible production flow must remain truthful.

Frontend migration packages must include a three-column table:

| Old path | Allowed intermediate state | Target path |
| --- | --- | --- |

## High-risk backend rules

Worker, queue, auth, RBAC, Provider, security, and state-machine tasks must include a failure-mode or state-transition matrix before coding begins.

The matrix must cover the relevant cases, such as:

- happy path
- duplicate delivery
- cancellation
- timeout
- retry
- stale claim or stale lock recovery
- enqueue failure
- third-party failure
- cross-tenant or unauthorized access
- sensitive-data redaction edge cases

If a case is intentionally out of scope, the package must say so explicitly.

## Control-plane and settings rules

Any task that adds settings, policy, or other control-plane APIs must include an explicit table that maps:

- each externally visible field
- the backend runtime consumer that makes it effective
- whether that consumer is already in scope for the task

If a field has no runtime consumer yet, it must not be exposed as active writable state. If making a field real requires changing a runtime consumer outside the allowed file scope, the task package is invalid as written and must be split or widened by the main agent before implementation starts.

Examples include default Provider/model selection, upload limits, concurrency limits, retention rules, feature flags, and billing policies.

## Child-agent responsibilities

- Do not treat unstated destructive behavior as permission. If the task would break an existing production path before its replacement exists, stop and report the conflict.
- Add tests for the named failure modes and migration invariants, not only happy paths.
- In the final handoff, map each required regression scenario to the actual test file and test name that covers it.
- State what was intentionally not changed and why.
- If a required invariant cannot be preserved inside the allowed file scope, report it instead of widening scope without approval.

## Main-agent review responsibilities

Review must explicitly check:

- whether required existing behavior was preserved
- whether the allowed intermediate state matches the task package
- whether any forbidden half-migrated state exists
- whether every named failure mode has a matching test or an explicit deferral
- whether the child-agent handoff maps required scenarios to real tests

When review finds a repeated class of omission, update the next task packages and these instructions so the rule becomes durable.
