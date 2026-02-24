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

Writing seed data by hand is boring, slow, and the data
looks fake. Generic tools like Faker don't understand your
schema — they don't know that order_items.product_id must
reference a real product, or that status can only be
'pending', 'paid', or 'shipped'.

`db-seed-ai` reads your actual schema, understands your
foreign keys and constraints, and generates data that looks
real and actually works.

## Demo
```bash
# Preview what gets generated (no database needed)
$ db-seed-ai preview --schema ecommerce.sql --table users --rows 5

# Seed your actual database
$ db-seed-ai seed --schema ecommerce.sql \
    --db postgres://localhost/mydb --rows 100

# SQLite (easiest for local dev)
$ db-seed-ai seed --schema schema.sql --db sqlite:./dev.db --rows 50
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
db-seed-ai preview --schema schema.sql --table users --rows 5
```

### seed — Generate and insert
```bash
db-seed-ai seed --schema schema.sql --db "postgres://localhost/mydb" --rows 100
db-seed-ai seed --schema schema.sql --db sqlite:./dev.db --rows 50
db-seed-ai seed --schema schema.sql --dry-run --rows 10
db-seed-ai seed --schema schema.sql --db sqlite:./dev.db --table users --rows 25
```

### validate — Check data quality
```bash
db-seed-ai validate --schema schema.sql --rows 10
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

- **Foreign keys** — order.user_id references real users
- **CHECK constraints** — status only gets allowed values
- **NOT NULL** — required columns never empty
- **UNIQUE** — emails, slugs never repeat
- **Data types** — prices decimals, dates timestamps

## Data Styles

- **realistic** (default) — Names, emails that look real. Good for demos.
- **minimal** — Short values, ASCII only. Good for unit tests.
- **edge-cases** — NULLs, boundary values, special chars. Good for QA.

## License
MIT
