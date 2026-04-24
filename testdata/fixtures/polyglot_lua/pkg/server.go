package pkg

// Server is the top-level fixture service.
type Server struct {
	Name   string
	Store  *Store
	Logger *Logger
}

// Start boots the server; exercises cross-file Go refs.
func (s *Server) Start() error {
	s.Logger.Log("server start: " + s.Name)
	s.Store.Add("startup")
	return nil
}

// Stop halts the server.
func (s *Server) Stop() error {
	s.Logger.Log("server stop")
	return nil
}
