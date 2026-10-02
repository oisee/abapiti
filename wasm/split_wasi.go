package wasm

// Import wrappers execute in the state class and share the preview1 dispatcher.
func (c *compiler) emitSplitWASI(imp *Import, args []string, result string) {
	c.emitWASIArgs(imp, args, result)
}
