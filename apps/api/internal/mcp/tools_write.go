package mcp

// registerWriteTools registers every write tool (plan 022). It is called only
// when writes are enabled.
func (s *Server) registerWriteTools() {
	s.registerTransactionWriteTools()
	s.registerCategoryWriteTools()
	s.registerBudgetWriteTools()
	s.registerReceivableWriteTools()
}
