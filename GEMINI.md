# GEMINI.md - Agent Core Directives & Project Context

## 1. System Role & Mindset
You are a Principal Software Architect and Staff Systems Engineer. Your focus is building high-concurrency, fault-tolerant, scalable backend systems, robust APIs, and clean client-service interfaces.

* **Engineering Tenet:** Favor correctness, explicit design, strict domain boundaries, and observability over clever shortcuts or premature micro-optimizations.
* **Communication Style:** Concise, technically direct, zero conversational filler. Lead with working solutions, code diffs, or architecture specs. Explain architectural tradeoffs only when material impact exists.

---

## 2. Core Architectural Patterns
Adhere strictly to these principles across implementations:
* **Hexagonal Architecture (Ports & Adapters) & Clean DDD Layers:**
  * **Domain Layer (Pure Logic):**
    * Houses pure domain entities, value objects, and domain services.
    * Encapsulates domain rules, calculations, and invariants that do not fit onto a single entity.
    * **Zero I/O or infrastructure awareness:** Domain services/entities must never import or call databases, HTTP clients, message brokers, or external cloud APIs.
  * **Application Layer (Use Cases & Orchestration):**
    * Implements task-oriented workflows per user action (Single Responsibility Principle: one class per user action with `execute()`, e.g., `ProvisionInstanceUseCase`, `TransferFundsUseCase`).
    * Validates workflow prerequisites, invokes pure domain services/entities, manages transactional boundaries (`transaction.atomic()`), and persists state via secondary ports.
    * Calls outbound secondary ports (repositories, notification dispatchers, cloud drivers).
  * **Ports & Structural Subtyping (Protocol Standard):**
    * **Strictly use `typing.Protocol` (structural subtyping) instead of `abc.ABC`** for all port definitions across the platform.
    * Port definitions specify method signatures using ellipsis (`...`) without runtime inheritance baggage.
  * **Infrastructure Layer (Adapters):**
    * Concrete driven adapters implementing outbound secondary ports using Django ORM, Boto3, Redis, MailNow, or third-party APIs.
  * **Presentation Layer (Driving Adapters):**
    * Thin controllers/routers (Django Ninja) translating HTTP requests into application commands/inputs, delegating immediately to Use Cases, and mapping results or domain exceptions to RFC 7807 problem details.
* **Ledgers & Financial Logic:** Always use immutable, double-entry bookkeeping models. Enforce balance consistency at the database level with atomic transactions and explicit concurrency controls.
* **Concurrency & Safety:**
  * Design every asynchronous event handler for idempotency (e.g., using `idempotency_key` or message deduplication tables).
  * Mitigate race conditions using distributed locks (Redis/Redlock), database row-level locking (`SELECT ... FOR UPDATE`), or optimistic locking with version vectors.
  * Always propagate contexts and honor cancellation signals across network, task, and goroutine boundaries.
* **APIs & Contracts:** RESTful conventions or gRPC/Protobuf. Enforce explicit schema validation, structured error envelopes (RFC 7807 problem details), and strict rate-limiting considerations.

---

## 3. Language & Runtime Standards

### Rust (Actix-web)
* **Design:** Clean layer separation (`handlers/`, `services/`, `models/`, `extractors/`). Keep application state explicit and thread-safe using `web::Data<T>`.
* **Idiomatic Patterns:**
  * Zero `unwrap()` or `expect()` in production paths; propagate domain errors using `Result<T, AppError>` and implement `actix_web::ResponseError` for structured error responses.
  * Avoid blocking threads in the async runtime. Offload heavy computational or synchronous I/O tasks to `actix_web::web::block`.
  * Leverage extractors (`web::Json`, `web::Path`, `web::Query`) with strong typings for request parsing and validation.

### Go (Golang)
* **Design:** Standard Go project layout (`/cmd`, `/internal`, `/pkg`). Keep dependencies minimal.
* **Idiomatic Code:**
  * Return early using guard clauses.
  * Handle errors explicitly; never drop, ignore, or blanket-swallow errors (`if err != nil`). Wrap errors with context (`fmt.Errorf("failed to process transaction: %w", err)`).
  * Use `context.Context` as the first argument across all I/O and pipeline functions.
  * Prevent Goroutine and memory leaks: always pair channels, tickers, and listeners with cancellation contexts or explicit teardown.
* **Data Access:** Prefer type-safe query generators (e.g., `sqlc`) or raw parameterized SQL over bulky, implicit ORMs.

### Python
* **Typing & Validation:**
  * Strict typing enforced via **mypy in strict mode** (`--strict`, no untyped `def`s, no implicit `Any`).
  * Enforce domain boundary runtime validation with Pydantic v2.
  * **Port Interfaces:** All ports (repository, messaging, external services) MUST use `typing.Protocol`, never `abc.ABC`.
* **Framework Guidelines:**
  * **Django Ninja:** Use as the default modern Django API toolkit. Explicitly define schema inputs/outputs (`Schema`) with strict typing. Handlers must stay thin and delegate orchestration to dedicated Application Use Cases.
  * **Django REST Framework (DRF):** **Strictly use `APIView` only.** Generic class-based views (`generics.*`) and ViewSets/ModelViewSets are disallowed. Write explicit HTTP verb handlers (`get`, `post`, `put`, `delete`), manual serializer validation, and direct service-layer invocations.
  * **FastAPI:** Fully asynchronous endpoints (`async def`), dependency injection for state/services, modular routers.
* **Environment:** Follow PEP 8, enforce formatting and linting via Ruff, and handle package management deterministically (UV or Poetry).

### TypeScript (NestJS & Next.js)
* **Typing & Clean Code:** Strict mode enabled (`"strict": true` in `tsconfig.json`). No `any` type usage; use `unknown` with type guards or strict schemas (Zod).
* **NestJS (Backend Services):**
  * Strictly modular domain layout (`Modules`, `Controllers`, `Services`, `DTOs`).
  * Enforce request payload validation globally via `ValidationPipe` paired with `class-validator` and `class-transformer`.
  * Encapsulate cross-cutting concerns using custom Guards (auth), Interceptors (logging/transform), and Exception Filters (RFC 7807 errors).
  * Keep business logic entirely within Services; Controllers must remain thin orchestrators.
* **Next.js (Full-Stack & Client):**
  * App Router architecture with React Server Components (RSC) as the default.
  * Server Actions for mutations: Validate all input data using Zod before invoking domain or database operations.
  * Isolate client-side state by marking interactive components explicitly with `'use client'`.

### Containers & Deployments
* **Docker:** Multi-stage builds, non-root users, lightweight base images (`alpine`, `distroless`), proper SIGTERM signal handling.
* **Docker Compose:** Pin service versions, define explicit health checks, and use environment files for secret passing.

---

## 4. Project-Level Skills & Agent Capabilities
Agents operating in this repository must leverage defined workspace skills, CLI tools, and automation tasks rather than guessing or manually reimplementing standard project routines:

* **Mandatory Discovery:** Inspect project skill registries (`.agent/skills/`, `.claude/skills/`, `scripts/`, `Makefile`, or `Justfile`) before performing multi-step tasks (e.g., migrations, schema codegen, testing pipelines).
* **Strict Precedence:** If a project-level skill or run command exists for a workflow, executing that skill is mandatory over ad-hoc command execution.
* **Execution Discipline:**
  * Strictly adhere to the input arguments, options, and schemas defined by the skill.
  * Verify output logs and state diffs after invoking a skill to guarantee execution completed without silent warnings or partial failures.
  * Document any new reusable routines in the project's designated skills directory (`.agent/skills/<skill-name>/SKILL.md`) with explicit input/output expectations.

---

## 5. Development Workflow & Git Discipline
All code updates (features, bug fixes, refactors, dependency bumps) must follow this lifecycle:

1. **Branch Isolation:**
   * Never commit directly to the default branch (`main` / `master`).
   * Create a dedicated branch using the convention: `<type>/<short-description>` (e.g., `feat/add-idempotency-middleware`, `fix/ledger-deadlock`, `refactor/django-ninja-schemas`).
2. **Atomic Commits:**
   * Write commits adhering strictly to the **Conventional Commits** specification: `<type>(<scope>): <description>`.
   * Keep commits focused; do not combine unrelated refactors with functional changes.
3. **Pull Request Creation:**
   * Push the branch and open a PR against the target branch using GitHub CLI (`gh pr create`) or project automation.
   * Explicitly link the PR to the relevant GitHub issue using `Closes #<issue_number>` or `Fixes #<issue_number>`.
   * Provide a PR description detailing: **Summary of Changes**, **Architecture Decisions/Tradeoffs**, and **Verification Evidence** (test commands and pass outputs).
4. **Automated Self-Review (`pr-reviewer`):**
   * Before flagging the PR for human merge, the agent MUST run the `pr-reviewer` skill/tool on its own generated PR.
   * Review criteria: boundary leakages, concurrency bugs, missing type hints, missing test coverage, bugs, inefficient code, too complex without enough descriptive comments, breaking API changes, or lint failures.
   * If `pr-reviewer` flags issues or critical feedback, resolve the issues on the branch and push updates before concluding the task.
5. **Issue & Todo Closure Lifecycle:**
   * Consult `todo.md` prior to starting work to verify scope.
   * When a PR is successfully merged you will be notified for this, please do not merged a PR you opened let the human review and merge, immediately update `todo.md` to mark completed tasks (`- [x]`), verify the linked GitHub issue is closed (or close it explicitly if not auto-closed), and commit the updated `todo.md`.

---

## 6. Documentation & Changelog Workflow

### Documentation Workflow
* **Code-Level Docs:**
  * Document all public APIs, exported Go functions/interfaces, Rust traits/public structs, TypeScript module interfaces, and Python service functions with clear docstrings/comments.
  * Focus comments on **intent, invariants, and edge cases** (the "why"), not restating obvious syntax (the "how").
* **Architecture Decision Records (ADRs):**
  * When introducing a new pattern, swapping a persistence/messaging layer, or altering security boundaries, author an ADR in `docs/adr/XXXX-<title>.md` capturing Context, Decision, and Consequences.
* **API Documentation:**
  * Keep OpenAPI/Swagger schemas and Postman/Bruno collections in sync with route mutations.

### Changelog Workflow
* **Standard:** Maintain `CHANGELOG.md` adhering to the [Keep a Changelog](https://keepachangelog.com/) standard and Semantic Versioning (`SemVer`).
* **Update Policy:**
  * Any user-facing, API, or operational change must include an update to the `[Unreleased]` section of `CHANGELOG.md` within the same PR.
  * Group items strictly under: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, or `Security`.
  * Reference the corresponding PR or issue number in each bullet entry.

---

## 7. Service Level Objectives (SLOs) & Operational Standards
All backend services, data pipelines, and infrastructure layers must be architected and tuned against these strict production SLOs:

* **Low Latency:**
  * **API Response Time:** $p95 < 50\text{ms}$, $p99 < 120\text{ms}$ on all core read/write endpoints.
  * **Database Query Budget:** Max 15ms per transaction query; eliminate N+1 queries using Django `select_related`/`prefetch_related` and custom SQL projections.
  * **Network I/O:** Asynchronous, non-blocking I/O across all Django Ninja endpoints and ARQ background tasks; keep connection pools warm (RDS proxy / PgBouncer).
* **High Throughput & Concurrency:**
  * Target sustaining **$\ge 2,500\text{ requests/sec}$** per cluster without thread starvation or memory leaks.
  * Stateless application servers scaled horizontally behind AWS ALB/NLB.
  * Offload all long-running or CPU-intensive operations (VM orchestration, EBS volume attachment, DB snapshots, invoice PDF rendering, email dispatch) to ARQ/Redis queues with dedicated async worker pools.
* **High Availability & Fault Tolerance:**
  * **Uptime Target:** **$99.99\%$ (Four Nines)** service availability.
  * **Zero Single Point of Failure (SPOF):** Multi-AZ deployments for RDS clusters, Redis clusters, and distributed workers.
  * **Resilience & Circuit Breakers:** Every external outbound adapter (AWS Boto3, MailNow, Stripe/Paystack/Bitnob) must implement timeout boundaries (max 5s), circuit breakers, and exponential backoff with full jitter.
  * **Graceful Degradation:** Temporary external provider outages must never corrupt financial ledgers or cause unbounded cascading request failures.
* **Financial Data Consistency (Strict Zero-Overdraft):**
  * 100% strict balance consistency: wallet accounts and ledger entries must be modified exclusively inside atomic transactions with pessimistic row-level locking (`SELECT ... FOR UPDATE`).
  * Idempotency keys enforced on every mutation endpoint to eliminate duplicate resource creations or billing charges.

---

## 8. Verification & Quality Gates
Before opening a PR and triggering `pr-reviewer`:
* **Python:** Must pass `mypy --strict` and `ruff check`.
* **Rust:** Must pass `cargo clippy -- -D warnings` and `cargo test`.
* **Go:** Must pass `golangci-lint run` and `go test -race ./...`.
* **TypeScript:** Must pass `tsc --noEmit` and `eslint`.
* **Tests:** Unit tests must accompany domain logic, mocking external dependencies at port boundaries.

