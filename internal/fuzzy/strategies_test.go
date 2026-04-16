package fuzzy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSweepExact(t *testing.T) {
	tests := []struct {
		name  string
		whole []string
		part  []string
		want  []int
	}{
		{"single hit", []string{"a", "b", "c", "d"}, []string{"b", "c"}, []int{1}},
		{"ambiguous 2 hits", []string{"x", "y", "z", "x", "y"}, []string{"x", "y"}, []int{0, 3}},
		{"no hit", []string{"a", "b", "c"}, []string{"z"}, nil},
		{"empty part", []string{"a", "b"}, []string{}, nil},
		{"part longer than whole", []string{"a"}, []string{"a", "b", "c"}, nil},
		{"1-line at EOF (off-by-one)", []string{"a", "b", "c"}, []string{"c"}, []int{2}},
		{"exact match whole-file", []string{"a", "b"}, []string{"a", "b"}, []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sweepExact(tt.whole, tt.part))
		})
	}
}

func TestSweepWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		whole []string
		part  []string
		want  []int
	}{
		{"leading ws ignored", []string{"  foo", "  bar"}, []string{"foo", "bar"}, []int{0}},
		{"trailing ws ignored", []string{"foo  ", "bar "}, []string{"foo", "bar"}, []int{0}},
		{"internal ws preserved (no match)", []string{"a  b"}, []string{"a b"}, nil},
		{"empty part", nil, []string{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sweepWhitespace(tt.whole, tt.part))
		})
	}
}

func TestSweepIndentFlex(t *testing.T) {
	tests := []struct {
		name  string
		whole []string
		part  []string
		want  []int
	}{
		{"tab vs spaces", []string{"\tfoo"}, []string{"    foo"}, []int{0}},
		{"spaces vs no indent", []string{"    foo"}, []string{"foo"}, []int{0}},
		{"internal ws still preserved", []string{"    a  b"}, []string{"a b"}, nil},
		{"trailing ws NOT stripped", []string{"foo "}, []string{"foo"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sweepIndentFlex(tt.whole, tt.part))
		})
	}
}
