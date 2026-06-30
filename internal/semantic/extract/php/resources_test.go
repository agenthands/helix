package phpextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectPhpResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`<?php
namespace App\Entity;

use Doctrine\ORM\Mapping as ORM;

/**
 * @ORM\Entity
 * @ORM\Table(name="users")
 */
class User
{
    /** @Id @Column(type="integer") */
    private $id;
}

/** @ORM\Entity */
class Account
{
}

#[ORM\Entity]
class Product
{
}

#[ORM\Table(name: "orders")]
#[ORM\Entity]
class Order
{
}

class Comment extends Model
{
}

class Tag extends \App\ActiveRecord
{
}

class PlainService
{
}

class ValueBag extends stdClass
{
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "entities.php", Language: "php"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct{ name, orm string }
	got := map[key]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[key{r.Name, r.ORM}] = r
	}

	cases := []struct {
		name  string
		orm   string
		table string
	}{
		{"User", "doctrine", "users"},   // @ORM\Entity docblock + @ORM\Table(name=...)
		{"Account", "doctrine", ""},     // @ORM\Entity docblock, no table
		{"Product", "doctrine", ""},     // #[ORM\Entity] attribute
		{"Order", "doctrine", "orders"}, // #[ORM\Table(name: ...)] attribute
		{"Comment", "eloquent", ""},     // extends Model
		{"Tag", "eloquent", ""},         // extends \App\ActiveRecord
	}
	for _, c := range cases {
		r, ok := got[key{c.name, c.orm}]
		if !ok {
			t.Errorf("missing resource %s (%s); detected=%+v", c.name, c.orm, ef.Resources)
			continue
		}
		if r.Table != c.table {
			t.Errorf("resource %s table = %q, want %q", c.name, r.Table, c.table)
		}
		if r.File != "entities.php" {
			t.Errorf("resource %s file = %q, want entities.php", c.name, r.File)
		}
		if r.Language != "php" {
			t.Errorf("resource %s language = %q, want php", c.name, r.Language)
		}
	}

	// False-positive guard: PlainService and ValueBag (extends stdClass) are
	// not entities and must not appear.
	if len(ef.Resources) != len(cases) {
		t.Errorf("resource count = %d, want %d (false positive); got %+v", len(ef.Resources), len(cases), ef.Resources)
	}
}
