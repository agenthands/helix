// Package price is the internal-toolbench/go fuzzy_search fixture
// (IT-go-fuzzy-search-1).
//
// Discount is DELIBERATELY WRONG: it returns the price unchanged. The scripted
// agent uses fuzzy_edit, whose whitespace-normalized strategy matches the
// `return price` body even when the supplied search text's indentation does not
// match byte-for-byte (D-05: the fuzzy match cascade is the exercised
// capability). After the edit Discount applies a 10% discount.
package price

// Discount should return price reduced by 10% (price * 9 / 10).
func Discount(price int) int {
	return price
}
