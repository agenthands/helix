package handler

// SetPopulateRecorderForTest installs a recorder populator on h. The
// populator is invoked AFTER UpsertOverlayFile and BEFORE Commit, on the
// recorder threaded through the tx span. Production code paths leave the
// underlying field nil; this setter is the single seam handler_test.go
// uses to simulate Phase 60 P04 / future type-resolver retrofit
// populators without pre-implementing those phases.
//
// 62-09 closure (truth #22): the populator is the contract bridge between
// today's empty-diff production state and the future populators that will
// fill the recorder in real flow.
func SetPopulateRecorderForTest(h *Handler, fn func(*FileFactDiffRecorder)) {
	if h == nil {
		return
	}
	h.populateRecorderForTest = fn
}
