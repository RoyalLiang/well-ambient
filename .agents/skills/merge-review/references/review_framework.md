# Evidence-first review framework

This framework combines the existing system review contract, merge-aware review, repository standards, specification compliance, and the ten engineering dimensions used by the Well Ambient review service.

## Finding qualification

A useful review finding is:

- introduced or exposed by the reviewed change;
- demonstrated by a real call path, state transition, data contract, or deployment path;
- tied to an exact changed `file:line`;
- important enough that the author would likely fix it;
- actionable without requiring the author to guess the cause;
- not already enforced more reliably by formatting, type checking, linting, or generated-code tooling.

Do not report:

- pre-existing defects outside the changed behavior;
- speculative failures without a reachable trigger;
- intentional changes that merely differ from reviewer preference;
- style nits that do not obscure behavior;
- business-rule claims without an applicable source;
- validation that was not actually run.

## Priority

| Priority | Meaning |
| --- | --- |
| P0 | Critical and broadly reproducible failure, security breach, irreversible data loss, or universal release blocker. |
| P1 | Urgent correctness, security, compatibility, migration, or operational defect that should block merge or release. |
| P2 | Concrete defect with a narrower trigger or impact that should be fixed. |
| P3 | Low-impact but actionable maintainability or usability defect. |

Severity follows impact and likelihood, not how difficult the fix is.

## Four review lenses

### 1. Defect

- Incorrect logic, state transitions, boundary handling, and error propagation.
- Concurrency races, lost updates, duplicate side effects, ordering, transactions, and idempotency.
- Authorization, injection, secret exposure, unsafe deserialization, and trust-boundary mistakes.
- Breaking API, event, schema, configuration, or serialized-data changes.
- Resource leaks, unbounded growth, N+1 work, blocking calls, and missing indexes.
- Migration, rollout, rollback, observability, and recovery failures.

### 2. Spec

- Every declared requirement is mapped to implementation evidence.
- Missing, partial, or behaviorally incorrect requirements are findings.
- Unrequested behavior and unrelated commits are scope findings when they increase risk.
- When no spec exists, say so. Do not reconstruct one from implementation alone.

### 3. Standards

Repository instructions override generic heuristics. Cite the exact standards source for hard violations.

Use these only as judgment-call smell prompts:

- Mysterious Name
- Duplicated Code
- Feature Envy
- Data Clumps
- Primitive Obsession
- Repeated Switches
- Shotgun Surgery
- Divergent Change
- Speculative Generality
- Message Chains
- Middle Man
- Refused Bequest

Skip smells that the repository explicitly accepts or that do not create a meaningful defect.

### 4. Engineering dimensions

Cover each dimension or state why it is not applicable:

1. **Business and requirements**: requirement alignment and business invariants. A business defect requires an applicable authoritative source.
2. **Robustness**: boundary inputs, failure recovery, timeout, cancellation, partial success, and resource lifetime.
3. **Reusability**: duplicated behavior, real reuse boundaries, and premature generalization.
4. **Abstraction**: interface contracts, dependency direction, and semantic level.
5. **Encapsulation**: state ownership, leaked implementation details, and responsibility boundaries.
6. **Concurrency and consistency**: idempotency, ordering, transactions, retries, and distributed consistency.
7. **Security and permissions**: authentication, authorization, data exposure, trust boundaries, and secrets.
8. **Performance and resources**: complexity, query shape, memory, I/O, connection, and storage growth.
9. **Testing and testability**: changed behavior coverage, failure paths, deterministic seams, and dependency isolation.
10. **Delivery and operations**: compatibility, migration, feature flags, observability, rollout order, and rollback.

## Evidence and line discipline

- Cite the merged/new-code line, not an old deleted line.
- Keep the cited range minimal and ensure it overlaps the reviewed diff.
- Read enough surrounding code to prove the trigger.
- Business findings cite knowledge/spec identifiers or exact source sections.
- If the line, call path, or knowledge source cannot be verified, downgrade it to a question.

## Independent counter-review

After drafting findings:

1. Re-read the relevant full files, callers, tests, and contracts.
2. Search for guards, retries, validation, or generated behavior that could invalidate the finding.
3. Check whether the issue existed before the target change.
4. Remove duplicates and merge findings with the same root cause.
5. Confirm priority against demonstrated impact.

The purpose of a second pass is to delete weak findings, not to increase the count.

## Coverage statement

Every report ends with:

- reviewed objects and refs;
- files or modules covered;
- checks actually executed;
- unread or truncated evidence;
- missing specs, knowledge, environment, or external-system validation;
- residual risk that remains after the review.
