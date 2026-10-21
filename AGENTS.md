## Commit & Pull Request Guidelines
Follow the lightweight conventional pattern seen in history (`fix:`, `feat:`, `chore:`) with concise, imperative summaries and optional scope (e.g., `fix: handle streaming fallback`). Reference related issues in parentheses `(#1234)` when helpful. PRs should describe the change, attach screenshots or GIFs for UI work, and list verification commands. Request review after lint/tests pass and diffs are free of secrets.

## Security & Configuration Tips
Never commit API keys or other credentials. Keel reads runtime configuration from environment variables (most names live in `constants/constants.go`), exposed through `chart/keel/values.yaml`; keep secret values out of those defaults. In `ui/`, only `.env.local` and `.env.*.local` are git-ignored - `ui/.env` is committed, so it must not hold secrets.

## Overthinking and excessive thoroughness

When you're deciding how to approach a problem, choose an approach and commit to it. Avoid revisiting decisions unless you encounter new information that directly contradicts your reasoning. If you're weighing two approaches, pick one and see it through. You can always course-correct later if the chosen approach fails.

Avoid over-engineering. Only make changes that are directly requested or clearly necessary. Keep solutions simple and focused:

- Scope: Don't add features, refactor code, or make "improvements" beyond what was asked. A bug fix doesn't need surrounding code cleaned up. A simple feature doesn't need extra configurability.

- Documentation: Don't add docstrings, comments, or type annotations to code you didn't change. Only add comments where the logic isn't self-evident.

- Defensive coding: Don't add error handling, fallbacks, or validation for scenarios that can't happen. Trust internal code and framework guarantees. Only validate at system boundaries (user input, external APIs).

- Abstractions: Don't create helpers, utilities, or abstractions for one-time operations. Don't design for hypothetical future requirements. The right amount of complexity is the minimum needed for the current task.

## Reduce file creation in agentic coding

If you create any temporary new files, scripts, or helper files for iteration, clean up these files by removing them at the end of the task.

## Avoid focusing on passing tests and hard-coding

Implement the actual logic so the solution works for all valid inputs, not just the test cases; don't hard-code values or special-case test inputs. Tests verify correctness, they don't define the solution.

If the task is infeasible or a test is wrong, say so instead of working around it.

Read the relevant code before answering questions about it or making claims about how it behaves; if the user names a file, open it first.
