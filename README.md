# hospital-middleware

A middleware API that lets hospital staff search patients from their hospital's
HIS (Hospital Information System). Staff belong to one hospital and can only see
that hospital's patients.

Go · Gin · PostgreSQL · Docker Compose · Nginx. Scaffolded with
[krok](https://github.com/DoIttikorn/krok) (gin + postgres).

Design documents (project structure, API spec, ER diagram) are in [`docs/`](docs):
[`PLAN.md`](docs/PLAN.md) and [`er-diagram.drawio`](docs/er-diagram.drawio).

## Run

Everything in Docker (nginx → app → postgres). Needs Docker only:

```bash
make up          # docker compose up --build -d; API on http://localhost (port 80)
make logs
make down
```

Settings have working dev defaults; copy `.env.example` to `.env` to change them
(for example `HTTP_PORT=8081` if port 80 is taken).

On your machine, with live reload (database in Docker):

```bash
make deps-up     # start db in Docker
make watch       # API on :8080 with live reload, or: make run
```

Migrations in [`migrations/`](migrations) run automatically at startup.

## Seed data (migrations)

Migration `000002_seed_master_data` creates the master data:

| Hospital code | Name | HIS |
| --- | --- | --- |
| `hospital-a` | Hospital A | `https://hospital-a.api.co.th` |
| `hospital-b` | Hospital B | none |

and one initial staff member per hospital, because `/staff/create` needs a login:

| username | password | hospital |
| --- | --- | --- |
| `admin` | `Admin@12345` | `hospital-a` |
| `admin` | `Admin@12345` | `hospital-b` |

**These credentials are for local/demo use only.** Change or delete them before
any real deployment. The `JWT_SECRET` default in `docker-compose.yml` is
dev-only too.

## API

Errors are [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details
(`application/problem+json`) with an extra machine-readable `code`.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/staff/login` | – | `{username, password, hospital}` → `{token, token_type, expires_in}` |
| `POST` | `/staff/create` | Bearer | `{username, password, hospital}` → `201 {id, username, hospital}`. Only for the caller's own hospital |
| `GET` | `/patient/search` | Bearer | Search the caller's hospital's patients |
| `GET` | `/livez`, `/readyz`, `/health` | – | Probes |

`/patient/search` query parameters, all optional: `national_id`, `passport_id`,
`first_name`, `middle_name`, `last_name`, `date_of_birth` (`YYYY-MM-DD`),
`phone_number`, `email`, `limit` (default 20, max 100), `offset`.
No criteria lists the hospital's patients. Names match the start of the Thai
**or** English name, ignoring case; the other fields match exactly. If a
`national_id` or `passport_id` is given and the patient is not stored yet, the
hospital's HIS is asked and the patient is saved.

| Status | `code` | When |
| --- | --- | --- |
| 400 | `validation_error`, `bad_request` | Invalid or malformed input |
| 401 | `unauthorized` / `invalid_credentials` | Missing or bad token / wrong login |
| 403 | `hospital_mismatch` | `/staff/create` for another hospital |
| 404 | `hospital_not_found` | Unknown hospital code |
| 409 | `username_taken` | Username exists in that hospital |
| 502 | `his_unavailable` | The HIS could not be reached |

Try it:

```bash
TOKEN=$(curl -s -X POST localhost/staff/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin@12345","hospital":"hospital-a"}' | jq -r .token)

curl -s -X POST localhost/staff/create -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"username":"nurse01","password":"s3cretPass!","hospital":"hospital-a"}'

curl -s "localhost/patient/search?first_name=som&limit=10" -H "Authorization: Bearer $TOKEN"
```

Or open [`api/api.http`](api/api.http) in VS Code with the REST Client
extension: every endpoint with its success and error cases, logins first.

`hospital-a.api.co.th` is not a live service here, so a search by
`national_id` for a patient that is not stored returns `502 his_unavailable`.
Point Hospital A at a mock with `HOSPITAL_A_BASE_URL`.

## Project layout

```
api/                     api.http: requests for trying the API by hand
cmd/api/                 main
internal/
  auth/                  JWT + bcrypt + Gin middleware
  his/                   HIS clients (Hospital A) + registry; histest = fakes
  hospital/  staff/  patient/   domains (see below)
  database/              pool, migration runner, dbtest helpers
  httpx/  logger/  config/  server/     from krok
migrations/              embedded SQL migrations + seed data
deploy/nginx/            nginx reverse proxy config
docs/                    plan, ER diagram
```

### Domains: ports and adapters

```
internal/patient/
├── patient.go        model, criteria, errors
├── service.go        business rules (Service): scoping, pagination, HIS import
├── repository.go     Repository port: the only view of storage
├── his.go            HISClient / HISRegistry ports
├── memory/           in-process adapter; also the test double
├── postgres/         postgres adapter
├── patienttest/      contract every Repository adapter must pass
└── handler/          REST adapter
```

`staff` and `hospital` follow the same layout. Domain packages import no
database driver and no web framework; adapters import the domain.
`internal/server/server.go` is the one place that picks adapters.

Domains talk to each other through services, never repositories.
`hospital.Service` is passed to `staff` and `patient`, and each declares a
`Hospitals` interface with only the methods it uses (`staff` needs `ByCode`,
`patient` needs `ByID`).

## Tests

```bash
make test               # unit tests (memory adapters, fake HIS, httptest)
make test-cover         # with coverage
make deps-up && make test-integration   # also runs the contracts against real postgres
```

Every API has positive and negative tests at handler level, service level and
end to end (`internal/server/api_test.go`). Each repository adapter passes the
same contract (`*test` packages): memory always, postgres with
`INTEGRATION=1`.

## Configuration

Read from environment variables; `.env` is loaded at startup.

| Variable | Default | |
| --- | --- | --- |
| `PORT` | `8080` | App port |
| `DB_HOST` / `DB_PORT` / `DB_DATABASE` / `DB_USERNAME` / `DB_PASSWORD` | `localhost` / `5432` / `hospital_middleware` / `app` / `secret` | Database |
| `JWT_SECRET` | required, ≥ 16 bytes | Token signing key |
| `JWT_TTL_MINUTES` | `60` | Token lifetime |
| `HIS_TIMEOUT_SECONDS` | `5` | Per-request HIS timeout |
| `HOSPITAL_A_BASE_URL` | – | Overrides the HIS address stored for `hospital-a` |
| `HTTP_PORT` | `80` | Host port nginx listens on (compose) |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` (`json` in Docker) | Logging |

## Make targets

| Target | Description |
| --- | --- |
| `make run` / `make watch` | Run the API / with live reload |
| `make build` | Build every command into `bin/` |
| `make test` / `make test-cover` / `make test-integration` | Tests |
| `make up` / `make down` / `make logs` | Everything in Docker Compose |
| `make deps-up` | Start only the database |
| `make docker-build` | Build the Docker image |
| `make tidy` / `make clean` | Tidy `go.mod` / remove build output |
