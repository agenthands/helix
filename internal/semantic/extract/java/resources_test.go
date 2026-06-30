package javaextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectJavaResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`package com.example;

@Entity
@Table(name = "users")
public class User {
    private Long id;
}

@Service
public class AccountService {
    public void doWork() {}
}

@Entity
public class Product {
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "Entities.java", Language: "java"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Map entity name → fact for lookup.
	got := map[string]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[r.Name] = r
	}

	cases := []struct {
		name, table, orm string
	}{
		{"User", "users", "jpa"}, // @Entity + @Table(name="users")
		{"Product", "", "jpa"},   // @Entity without @Table → table empty
	}
	for _, c := range cases {
		r, ok := got[c.name]
		if !ok {
			t.Errorf("missing entity %q; detected=%+v", c.name, got)
			continue
		}
		if r.Table != c.table {
			t.Errorf("entity %q table = %q, want %q", c.name, r.Table, c.table)
		}
		if r.ORM != c.orm {
			t.Errorf("entity %q orm = %q, want %q", c.name, r.ORM, c.orm)
		}
		if r.Language != "java" {
			t.Errorf("entity %q language = %q, want java", c.name, r.Language)
		}
		if r.File != "Entities.java" {
			t.Errorf("entity %q file = %q, want Entities.java", c.name, r.File)
		}
	}

	// No false positive: a @Service class is not a resource.
	if _, ok := got["AccountService"]; ok {
		t.Errorf("@Service class AccountService wrongly detected as resource; got=%+v", got)
	}
	if len(ef.Resources) != len(cases) {
		t.Errorf("resource count = %d, want %d; resources=%+v", len(ef.Resources), len(cases), ef.Resources)
	}
}
