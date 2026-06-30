package rubyextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectRubyResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`class User < ApplicationRecord
  has_many :posts
end

class Account < ActiveRecord::Base
end

class Webhook < ActiveRecord::Base::NoTouching
end

module Billing
  class Invoice < ApplicationRecord
    belongs_to :customer
  end
end

class Plain
end

class Helper < SimpleDelegator
end
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "models.rb", Language: "ruby"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct{ name, orm string }
	got := map[key]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[key{r.Name, r.ORM}] = r
	}

	cases := []struct {
		name string
		orm  string
	}{
		{"User", "activerecord"},    // < ApplicationRecord
		{"Account", "activerecord"}, // < ActiveRecord::Base
		{"Webhook", "activerecord"}, // < ActiveRecord::... subclass
		{"Invoice", "activerecord"}, // nested under module
	}
	for _, c := range cases {
		r, ok := got[key{c.name, c.orm}]
		if !ok {
			t.Errorf("missing resource %s (%s); detected=%+v", c.name, c.orm, ef.Resources)
			continue
		}
		if r.File != "models.rb" {
			t.Errorf("resource %s file = %q, want models.rb", c.name, r.File)
		}
		if r.Language != "ruby" {
			t.Errorf("resource %s language = %q, want ruby", c.name, r.Language)
		}
	}

	// False-positive guard: Plain (no superclass) and Helper (< SimpleDelegator)
	// are not ActiveRecord models and must not appear.
	if len(ef.Resources) != len(cases) {
		t.Errorf("resource count = %d, want %d (false positive); got %+v", len(ef.Resources), len(cases), ef.Resources)
	}
}
