package wasm

// WASI imports share buffers in the state class. Unsupported host operations
// return ENOSYS rather than silently claiming success.
func (c *compiler) emitSplitWASI(imp *Import, args []string, result string) {
	st := c.splitState
	switch imp.Name {
	case "fd_write":
		c.line("lv_wn = 0.")
		c.line("DO %s TIMES.", args[2])
		c.line("lv_wiov = %s + ( sy-index - 1 ) * 8.", args[1])
		c.line("lv_wptr = mem_ld_i32( lv_wiov ).")
		c.line("lv_wiov = lv_wiov + 4.")
		c.line("lv_wlen = mem_ld_i32( lv_wiov ).")
		c.line("lv_wbytes = %s=>mv_mem+lv_wptr(lv_wlen).", st)
		c.line("CONCATENATE %s=>mv_wasi_output lv_wbytes INTO %s=>mv_wasi_output IN BYTE MODE.", st, st)
		c.line("lv_wn = lv_wn + lv_wlen.")
		c.line("ENDDO.")
		c.line("mem_st_i32( iv_addr = %s iv_val = lv_wn ).", args[3])
		c.line("%s = 0.", result)
	case "fd_read":
		c.line("mem_st_i32( iv_addr = %s iv_val = 0 ).", args[3])
		c.line("%s = 0.", result)
	case "args_sizes_get", "environ_sizes_get":
		for _, a := range args {
			c.line("mem_st_i32( iv_addr = %s iv_val = 0 ).", a)
		}
		c.line("%s = 0.", result)
	case "args_get", "environ_get", "fd_close":
		c.line("%s = 0.", result)
	case "proc_exit":
		c.line("%s=>mv_wasi_exit = %s.", st, args[0])
		c.line(wasmTrap)
	default:
		if result != "" {
			c.line("%s = 52.", result)
		} else {
			c.line(wasmTrap)
		}
	}
}
