[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/rc-yile-dispatch/release.yml?branch=main&label=BUILD)](https://github.com/yylego/rc-yile-dispatch/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/rc-yile-dispatch)](https://pkg.go.dev/github.com/yylego/rc-yile-dispatch)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/rc-yile-dispatch/main.svg)](https://coveralls.io/github/yylego/rc-yile-dispatch?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26+-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/badge/release-active-blue.svg)](https://github.com/yylego/rc-yile-dispatch)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/rc-yile-dispatch)](https://goreportcard.com/report/github.com/yylego/rc-yile-dispatch)

# rc-yile-dispatch

API Notification Dispatch Service — accepts HTTP notification tasks from business systems and dispatches them to external APIs with at-least-once semantics.

---

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->

## CHINESE README

[中文说明](README.zh.md)

<!-- TEMPLATE (EN) END: LANGUAGE NAVIGATION -->

## Problem Understanding

Internal business systems need to send HTTP notifications to various external APIs (ad platforms, CRM systems, inventory services, etc.). Each vendor has different URLs, headers, and body formats. Business systems don't care about the response — they just need the notification to be sent.

The core challenge is **not the HTTP call itself**, but making it **reliable**: external APIs can be down, slow, or returning errors. We need a service that accepts the task, persists it, and keeps retrying until it succeeds.

## Architecture

```
Business System                    This Service                    External API
     |                                |                                |
     |  POST /api/dispatch            |                                |
     |  {url, headers, body}          |                                |
     |------------------------------->|                                |
     |                                |  1. Persist to SQLite          |
     |  {"id":1, "status":"pending"}  |                                |
     |<-------------------------------|                                |
     |                                |  2. Background Worker polls    |
     |                                |     pending tasks              |
     |                                |                                |
     |                                |  3. HTTP request  ------------>|
     |                                |                                |
     |                                |  4a. 2xx → mark success        |
     |                                |  4b. fail → exponential backoff|
     |                                |       retry until max retries  |
     |                                |  4c. max retries → deadline    |
```

## Design Decisions

### 1. System Boundaries

**What this service does:**

- Accept notification tasks via HTTP API
- Persist tasks to prevent data loss
- Dispatch HTTP requests to target URLs
- Automatic retries with exponential backoff
- Dead-letter status when max retries exceeded

**What this service does NOT do:**

- Parse or validate the external API response body — we check status code, that's it
- Transform request formats between systems — business systems must provide the exact URL/headers/body
- Rate limiting per vendor — out of scope in MVP, would add in v2
- Authentication management for external APIs — business systems include auth tokens in headers themselves

### 2. Delivery Semantics: At-Least-Once

The notification **may be sent more than once** (e.g., if the external API received the request but our service crashed before marking success). This is acceptable because:

- The requirement says business systems don't care about return values
- Most notification APIs are idempotent or tolerant of duplicates
- Exactly-once delivery requires distributed transactions, which is massive over-engineering in MVP

### 3. Failure Handling

**Exponential backoff retries:**

- Attempt 1 fails → wait 2s → attempt 2
- Attempt 2 fails → wait 4s → attempt 3
- Attempt 3 fails → wait 8s → attempt 4
- Up to configurable max retries (5 attempts, default)

**Dead-letter status:** After exhausting all retries, the task moves to `deadline` status. This means the external system has been unavailable too long — requires manual investigation. In a production system this would trigger an alert.

**What counts as failure:**

- Network error, timeout, connection refused
- HTTP response with non-2xx status code
- Request build failure (malformed URL, etc.)

### 4. Technology Choices and Trade-offs

| Choice            | Reasoning                                                 | Alternative (not used)                                          |
| ----------------- | --------------------------------------------------------- | --------------------------------------------------------------- |
| SQLite            | Single-binary deployment, zero config, good enough in MVP | PostgreSQL (production), Redis (volatile)                       |
| Database as queue | Simple, reliable, no extra infrastructure                 | RabbitMQ/Kafka (overkill in MVP)                                |
| Polling worker    | Simple implementation, predictable load                   | Goroutine-per-task (resource intensive), event-driven (complex) |
| `net/http`        | Standard lib, no framework overhead                       | Gin/Echo/Kratos (unnecessary abstraction in MVP)                |

**Why not use a message queue (Kafka/RabbitMQ)?**

- Adds operational complexity (deployment, monitoring, configuration)
- For MVP traffic levels, database polling is perfectly adequate
- The "queue" semantics are achieved through task status + NextRunAt in the database
- If traffic grows 100x, migrating to a dedicated queue is straightforward — the Store interface abstracts the persistence

**Why not use a web framework?**

- 3 endpoints don't justify a framework
- `net/http` + `ServeMux` is sufficient and has zero dependencies
- Demonstrates that the decision was made based on actual requirements, not habit

### 5. Future Evolution

If this becomes a production service with significant traffic:

1. **Phase 1: Horizontal scaling** — Switch SQLite to PostgreSQL, run multiple workers with row-level locking (`SELECT ... FOR UPDATE SKIP LOCKED`)
2. **Phase 2: Message queue** — Replace database polling with RabbitMQ/Kafka for higher throughput and lower latency
3. **Phase 3: Per-vendor configuration** — Add vendor registry with rate limits, circuit breakers, custom timeout settings
4. **Phase 4: Observability** — Prometheus metrics (dispatch latency, success rate, queue depth), distributed tracing

## API

### POST /api/dispatch — Submit a notification task

```json
{
  "method": "POST",
  "targetUrl": "https://external-api.example.com/webhook",
  "headers": {
    "Authorization": "Bearer xxx",
    "Content-Type": "application/json"
  },
  "body": "{\"event\":\"user_registered\",\"userId\":123}",
  "maxRetries": 5,
  "callback": "ad-system-registration"
}
```

Response:

```json
{ "id": 1, "status": "pending", "message": "task accepted" }
```

### GET /api/task?id=1 — Check task status

### GET /api/tasks?status=pending&page=1&pageSize=20 — List tasks

### GET /health — Service health check

## Quick Start

```bash
cd cmd
go run main.go
# Service starts on :8088

# Submit a test task
curl -X POST http://localhost:8088/api/dispatch \
  -H 'Content-Type: application/json' \
  -d '{"method":"POST","targetUrl":"https://httpbin.org/post","body":"{\"test\":true}"}'

# Check task status
curl http://localhost:8088/api/task?id=1
```

## Project Structure

```
rc-yile-dispatch/
├── cmd/main.go                  # Entry point: HTTP server + dispatcher startup
├── internal/
│   ├── model/task.go            # Task entity (database schema)
│   ├── store/store.go           # Database operations (CRUD + query)
│   ├── handler/submit.go        # HTTP handlers (submit/query tasks)
│   └── worker/dispatch.go       # Background worker (poll + dispatch + retry)
├── go.mod
└── README.md
```

## AI Usage

### Where AI helped

- Scaffolding the initial project structure and boilerplate code
- Writing the exponential backoff logic in store.MarkFailed
- Generating the HTTP handler with input validation

### AI suggestions NOT adopted

- AI suggested using Redis as a task queue — rejected because it adds infrastructure dependency without clear benefit at MVP scale. SQLite persistence is simpler and more reliable (no data loss on restart)
- AI suggested adding per-vendor configuration with YAML config files — rejected as over-engineering for MVP. Business systems already know the target URL/headers, no need to maintain a vendor registry
- AI suggested implementing circuit breaker pattern — rejected for MVP. Exponential backoff retries achieve similar protection without the complexity of state machine management

### Key decisions made by me

- **Database-as-queue pattern**: Based on experience building message dispatch systems. At low-to-medium traffic, a database with proper indexing (status + next_run_at) performs well and eliminates infrastructure dependencies
- **At-least-once semantics**: Chose not to attempt exactly-once because the business requirement explicitly states "don't care about return values" — duplicate notifications are harmless, but lost notifications are not
- **Single-process architecture**: One process with embedded SQLite means deployment is just copying a binary. No Docker compose, no service mesh, no configuration management. This matters when the goal is a first version that works
- **No vendor abstraction layer**: Business systems provide the complete request (URL + headers + body). Adding a vendor registry would assume we know all vendors upfront, which contradicts the "different vendors with different formats" requirement — let callers decide the format

---

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->

## 📄 License

MIT License - see [LICENSE](LICENSE).

---

## 💬 Contact & Feedback

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Mistake reports?** Open an issue on GitHub with reproduction steps
- 💡 **Fresh ideas?** Create an issue to discuss
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a merge request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉🎉🎉

<!-- TEMPLATE (EN) END: STANDARD PROJECT FOOTER -->
