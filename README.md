# db-seed-ai

> Stop writing fake data by hand. Let AI do it.

`db-seed-ai` reads your SQL schema file and uses a local
AI model (Ollama) to generate hundreds of rows of realistic,
relationally-correct seed data — then inserts it directly
into your database.

No cloud. No API keys. No cost. Runs 100% locally.

## The Problem

Every developer has done this:
```sql
INSERT INTO users VALUES (1, 'Test User', 'test@test.com');
INSERT INTO users VALUES (2, 'John Doe', 'john@doe.com');
-- ... repeat 50 more times, slowly dying inside
```


`db-seed-ai` reads your actual schema, understands your
foreign keys and constraints, and generates data that looks
real and actually works.

## Demo
![HCAZJh8bEAAP1RO](https://github.com/user-attachments/assets/bba2701e-a81f-4f15-8a83-47f28fbbf70b)

```bash
# Preview what gets generated (no database needed)
$ db-seed-ai preview --schema ecommerce.sql --table users --rows 5

  Asking llama3 to generate 5 rows...

  ┌────┬───────────────────┬───────────────────────┬───────────┐
  │ id │ name              │ email                 │ role      │
  ├────┼───────────────────┼───────────────────────┼───────────┤
  │  1 │ Sarah Mitchell    │ sarah.m@gmail.com     │ admin     │
  │  2 │ James Rodriguez   │ j.rodriguez@mail.com  │ user      │
  │  3 │ Priya Patel       │ priya.p@company.io    │ user      │
  │  4 │ Marcus Chen       │ m.chen@startup.dev    │ moderator │
  │  5 │ Emma Thompson     │ ethompson@email.net   │ user      │
  └────┴───────────────────┴───────────────────────┴───────────┘

  Dry run — no data was inserted.

# Seed your actual database
$ db-seed-ai seed --schema ecommerce.sql \
    --db postgres://localhost/mydb --rows 100

  db-seed-ai v0.1.0

  Schema loaded:  6 tables
  Insert order:   categories → users → products
                  → orders → order_items → reviews

  AI model: llama3

  Generating seed data...

    Generating  categories          ✓ 10 rows
    Generating  users               ✓ 100 rows
    Generating  products            ✓ 100 rows
    Generating  orders              ✓ 100 rows
    Generating  order_items         ✓ 200 rows
    Generating  reviews             ✓ 150 rows

  Inserting into database...

    Inserting   categories          ✓ 10 inserted
    Inserting   users               ✓ 100 inserted
    Inserting   products            ✓ 100 inserted
    Inserting   orders              ✓ 100 inserted
    Inserting   order_items         ✓ 200 inserted
    Inserting   reviews             ✓ 150 inserted

  ✓ Done in 4.2s — 660 rows inserted across 6 tables
```

## Installation
```bash
# Prerequisites
# 1. Install Go 1.22+ from https://go.dev/dl
# 2. Install Ollama from https://ollama.ai/download
# 3. Pull a model
ollama pull llama3

# Install db-seed-ai
git clone https://github.com/satyammistari/db-seed-ai
cd db-seed-ai
go build -o db-seed-ai .
```

## Commands

### preview — See data before touching your DB
```bash
db-seed-ai preview \
  --schema schema.sql \
  --table users \
  --rows 5
```

### seed — Generate and insert
```bash
# PostgreSQL
db-seed-ai seed \
  --schema schema.sql \
  --db "postgres://localhost/mydb" \
  --rows 100

# SQLite (easiest for local dev)
db-seed-ai seed \
  --schema schema.sql \
  --db sqlite:./dev.db \
  --rows 50

# Dry run — generate but do not insert
db-seed-ai seed \
  --schema schema.sql \
  --dry-run \
  --rows 10

# One specific table only
db-seed-ai seed \
  --schema schema.sql \
  --db sqlite:./dev.db \
  --table users \
  --rows 25
```

### validate — Check data quality
```bash
db-seed-ai validate \
  --schema schema.sql \
  --rows 10
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| --schema | required | Path to your .sql schema file |
| --db | required | Database connection string |
| --rows | 100 | Rows to generate per table |
| --table | all tables | Only seed this one table |
| --model | llama3 | Ollama model to use |
| --style | realistic | realistic, minimal, edge-cases |
| --batch-size | 500 | Rows per INSERT batch |
| --dry-run | false | Generate but do not insert |

## What It Understands

`db-seed-ai` parses your schema and passes this context
to the AI so it generates valid data:

- **Foreign keys** — order.user_id always references a
  real user that was already inserted
- **CHECK constraints** — status only ever gets values
  from ('pending', 'paid', 'shipped')
- **NOT NULL** — required columns are never empty
- **UNIQUE** — emails, slugs, and usernames never repeat
- **Data types** — prices are decimals, dates are
  timestamps, IDs are integers

## Data Styles

**realistic** (default) — Names, emails, and text that
looks like it came from a real app. Good for demos.

**minimal** — Short values, ASCII only, no special
characters. Good for unit tests.

**edge-cases** — Includes NULL values, near-max-length
strings, boundary numbers, and special characters.
Good for QA testing.


## Architecture

Six components, one job each:

- `cmd/`         CLI commands (seed, preview, validate)
- `schema/`      Parses your SQL file into Go structs
- `generator/`   Builds AI prompt, calls Ollama, parses JSON
- `inserter/`    Writes rows to Postgres or SQLite
- `validator/`   Checks rows against your constraints
- `reporter/`    Colored terminal output and progress

## Engineering Trade-offs

**1. Topological sort for insert order**
Foreign keys mean you can't insert orders before users
exist. We build a dependency graph from your schema
and do a DFS topological sort. Categories go in first,
products second (they reference categories), orders
third (they reference users), and so on. Circular FKs
are detected and reported as errors.

**2. Transactions per batch**
Every batch of 500 rows is wrapped in a BEGIN/COMMIT.
If one batch fails, only that batch rolls back — not
the entire run. This means a 10,000 row seed that fails
at row 8,000 doesn't waste the first 7,999 rows.

**3. Fetch existing IDs before generating**
Before asking the AI to generate rows for `orders`, we
run `SELECT id FROM users LIMIT 1000` and pass those
real IDs to the AI. This is what makes FK relationships
actually work instead of generating IDs that don't exist.

## Contributing
```bash
go mod tidy
go test ./...
go build .
```

## License
MIT
