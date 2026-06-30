package goextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestDetectGoResources covers GORM entity detection: a struct whose fields
// carry `gorm:"..."` tags (and a gorm.Model embed) is detected, while a plain
// struct with no gorm tags is NOT. Tree-sitter parses syntax only, so the
// gorm import/type need not resolve — the tag substring is what matters.
func TestDetectGoResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`package main

type User struct {
	ID        uint   ` + "`gorm:\"column:id;primaryKey\"`" + `
	Email     string ` + "`gorm:\"column:email\"`" + `
	Name      string ` + "`gorm:\"column:name;tableName:accounts\"`" + `
	gorm.Model
}

// Plain struct — no gorm tags, must NOT be detected.
type Point struct {
	X int
	Y int
}

// Plain struct embedding a non-gorm type — must NOT be detected.
type Order struct {
	base.Base
	Total int
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.go", Language: "go"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Exactly one resource: the User struct.
	var got *extract.ResourceFact
	for i := range ef.Resources {
		if ef.Resources[i].Name == "User" {
			got = &ef.Resources[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("User resource not detected; got=%+v", ef.Resources)
	}

	if got.ORM != "gorm" {
		t.Errorf("User ORM = %q, want %q", got.ORM, "gorm")
	}
	if got.Table != "accounts" {
		t.Errorf("User Table = %q, want %q (from gorm tableName: directive)", got.Table, "accounts")
	}

	// No false positives: Point and Order must not appear.
	for _, r := range ef.Resources {
		if r.Name == "Point" || r.Name == "Order" {
			t.Errorf("plain struct %q must not be detected as a resource: %+v", r.Name, r)
		}
	}
	if len(ef.Resources) != 1 {
		t.Errorf("expected exactly 1 resource, got %d: %+v", len(ef.Resources), ef.Resources)
	}
}
