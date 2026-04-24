package pkg

// Store holds fixture state.
type Store struct {
	items  []string
	Logger *Logger
}

// Add appends an item.
func (s *Store) Add(item string) {
	s.items = append(s.items, item)
	s.Logger.Log("store add: " + item)
}

// Get returns the item at i.
func (s *Store) Get(i int) string {
	if i < 0 || i >= len(s.items) {
		return ""
	}
	return s.items[i]
}

// Delete removes all items.
func (s *Store) Delete() {
	s.items = nil
	s.Logger.Log("store delete")
}
