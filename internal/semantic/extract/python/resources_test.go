package pyextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectPyResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`from sqlalchemy import Column, Integer, String
from sqlalchemy.ext.declarative import declarative_base
Base = declarative_base()


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    name = Column(String)


class Plain:
    """Not an ORM entity — no __tablename__."""

    def method(self):
        pass


def helper():
    pass
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "models.py", Language: "python"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Map entity name -> table for easy assertions.
	got := map[string]string{}
	for _, r := range ef.Resources {
		got[r.Name] = r.Table
	}

	// Positive: the SQLAlchemy entity is detected with its table name.
	if tbl, ok := got["User"]; !ok {
		t.Errorf("User resource not detected; got=%v", got)
	} else if tbl != "users" {
		t.Errorf("User table = %q, want %q", tbl, "users")
	}

	// Negative: plain class / plain function are NOT resources.
	if _, ok := got["Plain"]; ok {
		t.Errorf("Plain class must not be a resource; got=%v", got)
	}
	if _, ok := got["helper"]; ok {
		t.Errorf("helper function must not be a resource; got=%v", got)
	}

	// Exactly one resource, with correct metadata.
	if len(ef.Resources) != 1 {
		t.Fatalf("expected exactly 1 resource, got %d (%v)", len(ef.Resources), got)
	}
	r := ef.Resources[0]
	if r.ORM != "sqlalchemy" {
		t.Errorf("User ORM = %q, want %q", r.ORM, "sqlalchemy")
	}
	if r.Language != "python" {
		t.Errorf("User Language = %q, want %q", r.Language, "python")
	}
	if r.File != "models.py" {
		t.Errorf("User File = %q, want %q", r.File, "models.py")
	}
	if r.Range.Start.Line != 5 { // class User on the 6th source line (0-based 5)
		t.Errorf("User Range.Start.Line = %d, want 5", r.Range.Start.Line)
	}
}
