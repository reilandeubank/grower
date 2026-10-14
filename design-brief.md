# Personal Plant Care App: Design Brief

Background and constraints for producing an implementation plan. This is not a plan. The planning agent should turn it into phased milestones, a schema, an API outline, and a task breakdown, and should flag anything here it thinks is wrong or underspecified. Details below are leanings, not requirements, unless marked as constraints.

## 1. Purpose

A self-hosted web app for tracking houseplants and their care, for one person (the owner), running on a home server. It replaces two options:

- **Planta**: good UX and care logic, but heavily paywalled and not self-hostable.
- **HortusFox** (open source, PHP): self-hostable, but slow development, dated UI, and PRs are now disabled, so forking is a dead end.

This is a from-scratch build with a similar feature set, not a code fork. Use both apps as feature references only.

Secondary goal: a learning exercise in deploying and operating a self-hosted app (containers, reverse proxy, backups, updates). Favor transparency and learnability in deployment choices.

## 2. Goals and non-goals

**Goals**

- Modern, fast, mobile-first UI that feels app-like on a phone.
- A clear "what needs care today" core loop: open the app, see what's due, tap done.
- Reliable reminders via push notifications.
- Photo timeline per plant.
- Low operational burden: one container, one small database, easy backup.
- All data stays local; external services are optional.

**Non-goals (initially)**

- Multi-tenant SaaS, public signup, billing.
- Native mobile apps (an installable PWA is enough).
- Social features, or competing with Planta's proprietary plant database.

## 3. Context (constraints)

- Primary user: the owner, mostly on an **Android phone** (PWA installed to the home screen), sometimes on a laptop.
- Exposed to the internet through a **reverse proxy with TLS**, so the app needs real authentication of its own and must behave correctly behind a proxy (forwarded headers, secure cookies). A household second user may come later; don't build a permission system for it.
- Hosted on **TrueNAS SCALE** as a Docker Compose custom app.

## 4. Feature scope

**MVP (core loop)**

- Plants: name, species (free text at first), location, acquired date, notes, cover photo.
- Locations (rooms/spots) with a light-level note.
- Recurring care tasks per plant (watering, fertilizing, custom) with interval-based schedules.
- "Today" view: overdue and due tasks, one-tap complete and snooze.
- Care history per plant.
- Photo upload from the phone, server-side thumbnails, per-plant timeline.
- Single-user login with sessions that persist well on mobile.

**Phase 2**

- Web push reminders (daily digest and/or per task).
- Species care defaults pulled from an external plant API and cached locally (e.g. Perenual, Trefle, OpenPlantbook; the agent should check current availability and licensing). The app must stay fully usable with no external data.
- Simple seasonal adjustment of watering intervals.
- Quick-add via photo, with optional species identification.
- Export/import (JSON + photos).

**Later / maybe**: plant health notes, propagation tracking, calendar (ICS) feed, Home Assistant integration.

## 5. Domain model (rough sketch)

Plant, Location, Species (locally cached care defaults, user-editable), CareTask (type, interval, snooze), CareEvent (append-only log of what was done and when), Photo, User/Session, PushSubscription.

Design notes:

- Scheduling logic (next-due, snooze, timezone and DST handling, seasonal adjustment) is the part most likely to fail quietly. Isolate it in a pure, well-tested module.
- Store UTC; evaluate "due today" against the user's local day.
- Derive next-due from the event log rather than mutating counters.

## 6. Architecture constraints

- **Single deployable**: one container, ideally one process serving API, static frontend, and uploaded media.
- **Database**: SQLite by default (single-user scale), with real migrations from day one and a data layer clean enough to move to Postgres later if needed.
- **Storage**: photos on local disk. Everything stateful (DB + media) lives under one mounted data directory so backup is trivial.
- **API**: a typed, documented contract (e.g. OpenAPI). If frontend and backend are different languages, generate the client types from it.
- **PWA**: installable, service worker, web push (works well on Android Chrome with the installed PWA).
- **Background jobs**: in-process scheduler for push dispatch and cache refresh, with job state in the DB so restarts don't drop or duplicate notifications.
- Basic structured logging and a health endpoint.

## 7. Implementation language and stack

The agent should pick one and justify it, defaulting to Option A.

**Option A (preferred): Go backend + embedded SPA.** Go with a pure-Go SQLite driver (no CGO), a typed query approach, and a Vite-built React or SvelteKit SPA embedded in the binary. One binary, small image, low memory. The owner enjoys Go. Cost: two languages, mitigated by generated API types.

**Option B: Next.js full-stack (TypeScript).** Single language and fast UI iteration, which the owner has experience with. Cost: heavier runtime, and background jobs and push scheduling fit awkwardly in the server model.

Either way, use TypeScript on the frontend and put real thought into visual design rather than a default-template look. A modern UI is a primary motivation for the project.

## 8. Deployment

- App data directory on a dedicated ZFS dataset with periodic snapshots, plus a documented off-box backup.
- Git repo with CI that builds, tests, and publishes a container image (e.g. to GHCR), and a simple documented update path on the server.
- Deploy a bare skeleton in the first milestone so the pipeline is exercised early.

## 9. Process expectations

- The app will be built largely with AI coding agents, so the repo should be agent-friendly: a short conventions file at the root, small well-named modules, fast tests, typed boundaries.
- Test priorities: the scheduling module (edge cases like DST, snooze, missed days), API integration tests against a temp SQLite DB, and a few end-to-end smoke tests of the core loop.
- Few, mainstream dependencies; the owner is sole maintainer.
- Build in vertical slices: one thin feature working end to end, and deployed, before broadening.

## 10. Risks and open questions

- Species data quality and licensing from free APIs.
- Photo storage growth over time (retention of originals vs. thumbnails).
- Scope creep, especially UI polish. The core loop comes first.
- Auth details, given public exposure (local account only, or also reverse-proxy/SSO header support).
- Define an export format early so data is never locked in.

## 11. What the planning agent should produce

1. A stack recommendation (A or B) with brief rationale and any objections to this brief.
2. A phased milestone plan, where milestone 1 includes a deployed skeleton.
3. A schema and migrations approach.
4. An API outline and screen map, with "Today" as the primary screen.
5. A scheduling-logic design covering the edge cases above.
6. Repo layout, conventions file contents, test strategy, and CI/CD outline.
7. A list of decisions that still need input from the owner.
8. (If applicable) use (or create a plan to use) Claude Design to produce a visual design system and component library for the frontend, with a focus on mobile-first UX and ease of use.
