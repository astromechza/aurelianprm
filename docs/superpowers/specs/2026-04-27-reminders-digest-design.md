# Reminders Digest — Design Spec

## Goal

Add a `send-digest` CLI subcommand that queries the database for upcoming reminders and sends a plain-text email digest. If no reminders are found, no email is sent. V1 implements birthday reminders only; the architecture is designed for additional reminder sources in future.

## Architecture

New `internal/digest/` package owns all reminder logic. `main.go` gains subcommand dispatch: first arg `send-digest` runs the digest, otherwise the HTTP server starts as before (backwards compatible). SMTP credentials and recipient are provided via environment variables. No new database migration is required.

**File map:**

| File | Action | Purpose |
|------|--------|---------|
| `main.go` | Modify | Subcommand dispatch |
| `internal/digest/digest.go` | Create | `Reminder` type, `FindReminders`, email compose + send |
| `internal/digest/birthday.go` | Create | Birthday reminder source (v1) |
| `internal/digest/digest_test.go` | Create | Unit tests |

---

## Data Model

No migration. Reads existing `entities` table via `dal.Queries`. Birthday fields live in the JSON `data` column of Person entities (`birthMonth`, `birthDay`, `birthYear`).

### `Reminder` type

```go
type Reminder struct {
    Date  time.Time
    Label string // e.g. "Alice Smith's birthday (turning 35)"
}
```

---

## Reminder Sources

### V1: Birthdays

**Window:** today through today+6 inclusive (7 days, today counts).

**Logic:**
- Load all entities with `type = 'Person'` via `dal.ListPersons` (or equivalent).
- Decode each entity's JSON data into `PersonData`.
- Skip persons where `BirthMonth == 0 || BirthDay == 0`.
- For each candidate, compute this year's birthday (and next year's if this year's already passed). Check whether the birthday falls within [today, today+6].
- If yes, construct a `Reminder`. Include age ("turning N") if `BirthYear > 0`.

**Leap day (Feb 29):** if born on Feb 29 and current year is not a leap year, use Mar 1 as the effective birthday date for reminder purposes.

---

## Email Format

**Subject:** `Reminders for <Mon 28 Apr 2026>`

**Body (plain text):**
```
Upcoming reminders in the next 7 days:

• Mon 28 Apr — Alice Smith's birthday (turning 35)
• Wed 30 Apr — Bob Jones's birthday
```

Reminders sorted by date ascending, then by label alphabetically.

No email is sent if the reminder list is empty.

---

## SMTP Configuration (environment variables)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SMTP_HOST` | Yes | — | SMTP server hostname (e.g. `smtp.fastmail.com`) |
| `SMTP_PORT` | No | `587` | SMTP port |
| `SMTP_USER` | Yes | — | SMTP username (full email address for Fastmail) |
| `SMTP_PASS` | Yes | — | SMTP password. For Fastmail with 2FA, use an **app password**, not the account password. |
| `SMTP_FROM` | Yes | — | Sender address |
| `SMTP_SSL` | No | `false` | Set `true` for implicit TLS (port 465). Default uses STARTTLS mandatory (port 587). |
| `DIGEST_EMAIL_ADDRESS` | Yes | — | Recipient address |

Library: `github.com/wneessen/go-mail`. Handles TLS/STARTTLS automatically based on `SMTP_SSL`.

`send-digest` exits non-zero and prints an error if any required variable is missing or if sending fails.

---

## CLI Usage

```sh
# Serve (unchanged)
aurelianprm --db ./aurelianprm.db

# Send digest
SMTP_HOST=smtp.fastmail.com \
SMTP_PORT=465 \
SMTP_USER=you@fastmail.com \
SMTP_PASS=app-password \
SMTP_FROM=you@fastmail.com \
SMTP_SSL=true \
DIGEST_EMAIL_ADDRESS=you@example.com \
aurelianprm send-digest --db ./aurelianprm.db
```

Suitable for a daily cron job or Kubernetes CronJob.

---

## Testing

| Test | What it covers |
|------|----------------|
| `TestFindBirthdayReminders` | Table-driven: today's birthday, tomorrow's, day 6 (included), day 7 (excluded), no birthday set, unknown birth year (no age), leap day Feb 29 |
| `TestSendDigest_noReminders` | No email sent when list empty |
| `TestSendDigest_withReminders` | Correct subject, body, recipient via `go-mail` mock transport (`SendFunc`) |

No live SMTP in tests.

---

## Deployment

### Recommended schedule

Run once daily at 08:00 local time (`0 8 * * *`).

### Docker / host cron

```sh
# crontab entry
0 8 * * * docker run --rm \
  --env-file /etc/aurelianprm/smtp.env \
  -v aurelianprm_data:/data \
  ghcr.io/astromechza/aurelianprm:latest \
  send-digest --db /data/aurelianprm.db
```

`smtp.env` file (keep out of version control):
```
SMTP_HOST=smtp.fastmail.com
SMTP_PORT=465
SMTP_USER=you@fastmail.com
SMTP_PASS=app-password
SMTP_FROM=you@fastmail.com
SMTP_SSL=true
DIGEST_EMAIL_ADDRESS=you@example.com
```

### Kubernetes CronJob

Store SMTP credentials in a Secret, mount the same PVC used by the StatefulSet:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aurelianprm-smtp
stringData:
  SMTP_HOST: smtp.fastmail.com
  SMTP_PORT: "465"
  SMTP_USER: you@fastmail.com
  SMTP_PASS: app-password        # use a Fastmail app password
  SMTP_FROM: you@fastmail.com
  SMTP_SSL: "true"
  DIGEST_EMAIL_ADDRESS: you@example.com
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: aurelianprm-digest
spec:
  schedule: "0 8 * * *"          # 08:00 UTC daily
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          securityContext:
            runAsUser: 65532
            runAsGroup: 65532
          containers:
            - name: digest
              image: ghcr.io/astromechza/aurelianprm:latest
              args: ["send-digest", "--db", "/data/aurelianprm.db"]
              envFrom:
                - secretRef:
                    name: aurelianprm-smtp
              volumeMounts:
                - name: data
                  mountPath: /data
                  readOnly: true   # digest only reads the DB
              resources:
                requests:
                  cpu: 10m
                  memory: 32Mi
                limits:
                  memory: 64Mi
          volumes:
            - name: data
              persistentVolumeClaim:
                claimName: data-aurelianprm-0   # PVC created by the StatefulSet
```

The CronJob mounts the PVC read-only since `send-digest` only reads the database.

---

## Extensibility

To add a new reminder source in future (e.g. note-based follow-ups):
1. Create `internal/digest/notes.go` with a function matching the source signature.
2. Register it in `FindReminders` alongside `findBirthdayReminders`.
No changes to email/send logic required.
