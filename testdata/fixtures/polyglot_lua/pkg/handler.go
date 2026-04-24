package pkg

// Handler wires together Server, Store, and Logger.
type Handler struct {
	Server *Server
	Store  *Store
	Logger *Logger
}

// Handle exercises cross-file calls into every other Go type.
func (h *Handler) Handle(msg string) error {
	h.Logger.Log("handle " + msg)
	if err := h.Server.Start(); err != nil {
		h.Logger.Error(err.Error())
		return err
	}
	h.Store.Add(msg)
	return h.Server.Stop()
}
