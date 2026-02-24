package schema

import (
	"testing"
)

func TestParseFile(t *testing.T) {
	sql := `
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  role VARCHAR(50) CHECK (role IN ('admin', 'user'))
);
CREATE TABLE orders (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES users(id)
);
`
	tables, err := ParseFile(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}
	// Topological order: users before orders (orders references users)
	if tables[0].Name != "users" {
		t.Errorf("first table should be users, got %s", tables[0].Name)
	}
	u := tables[0]
	if len(u.Columns) != 3 {
		t.Fatalf("users: expected 3 columns, got %d", len(u.Columns))
	}
	if u.Columns[0].Name != "id" || !u.Columns[0].PrimaryKey {
		t.Errorf("users.id should be primary key")
	}
	if len(u.Columns[2].CheckIn) != 2 {
		t.Errorf("users.role CheckIn expected 2 values, got %v", u.Columns[2].CheckIn)
	}
	o := tables[1]
	if o.Columns[1].ForeignKey == nil || o.Columns[1].ForeignKey.RefTable != "users" {
		t.Errorf("orders.user_id should reference users")
	}
}
