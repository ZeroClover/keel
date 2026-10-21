# Web dashboard (`ui/`)

Verified against commit `9070252f` (2026-10-22). Requirements: `openspec/specs/web-dashboard/spec.md`.

## Stack

Vue 2 (`vue ^2.5`), ant-design-vue 1.3, vue-router 3, vuex 3, vue-resource, `@websanova/vue-auth`, built with `@vue/cli-service` 3, ESLint 5 and Jest 23. The project started from the Ant Design Pro Vue template (package name `vue-antd-pro`), so some template code remains unused.

## Commands

Use yarn; `yarn.lock` is the lockfile CI caches and the Dockerfile installs from. `package-lock.json` is stale.

| Task | Command |
|---|---|
| Install | `cd ui && yarn` |
| Dev server (port 8000) | `yarn run serve` |
| Lint (what CI runs) | `yarn run lint --no-fix` |
| Production build → `ui/dist` | `yarn run build` |
| Unit tests | `yarn run test:unit` |

Node 16 matches CI and the Dockerfile.

## How it talks to Keel

- API base: `<origin>/v1` in production, `http://localhost:9300/v1` in development (`ui/src/main.js`). Log in with `POST /v1/auth/login`, then Bearer tokens via vue-auth.
- Keel serves the built files only when basic auth is configured (`BASIC_AUTH_USER` and `BASIC_AUTH_PASSWORD`); without it there is no admin API and no UI. Routes are listed in `docs/agents/configuration.md` → HTTP server.
- The production image copies `ui/dist` to `/www`; `make run` serves `ui/dist` directly.
- Changing a policy calls `PUT /v1/policies`; changing tracking calls `PUT /v1/tracked`. Neither endpoint validates the value against `internal/policy`, so the UI is responsible for only offering valid policies.

## Known gaps

The policy menu (`ui/src/views/dashboard/Analysis.vue`) still offers the removed legacy policies and can't select the current ones, and `ui/README.md` describes removed features. Details in `docs/agents/known-issues.md`.
