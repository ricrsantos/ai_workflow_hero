# Grilling Skill — Structured Requirements Elicitation

## When to Use

Use this skill during the Research stage when conducting requirements gathering sessions with the user. The goal is to stress-test a plan or design through relentless questioning before committing to implementation.

## Grilling Protocol

1. Ask one focused question at a time — never overwhelm the user with a list.
2. Follow up on every vague answer; do not accept "it depends" without clarification.
3. Explore all relevant dimensions:
   - **What**: What exactly needs to be built?
   - **Why**: What problem does it solve?
   - **Who**: Who uses it and in what context?
   - **When**: What are the timeline and priority constraints?
   - **How**: Are there architectural or technology constraints?
   - **What if**: What are the edge cases and failure modes?
4. Stop grilling only when all key requirements are unambiguous and agreed upon.
5. Summarize the agreed requirements at the end for confirmation.
6. **Pre-document gate:** Before creating any documents, ask the user whether they want to add any more information about the project. Wait for their reply; do not generate documents in the same message as this question.
7. If the user adds information, the `discover_agent` evaluates the additions (scope, constraints, trade-offs) and incorporates them into the requirements used for document generation. Ask short follow-ups only if the additions create material ambiguity.
8. Only after the gate (and any follow-ups) is resolved, create the appropriate documents (PRD, ADR, UI spec, etc.) and register them in `documents.json`.

## Output

After the grilling session **and** the Pre-document gate, the discover_agent creates the appropriate documents (PRD, ADR, UI spec) and registers them in `documents.json`.

## Tone

Be direct and persistent, but not adversarial. The goal is clarity, not confrontation.
