package tsfront

import "github.com/oisee/abapiti/hir"

// Synthetic forwarders precede body lowering. Union discovery may subsequently
// rename their targets or widen their physical results. Bind the final target
// ABI before verification, then recover a checked reference view at the return
// boundary. Narrow preserves identity and optional absence on both backends.
func (l *lowerer) completeReturnBridges() {
	visited := map[*hir.Method]bool{}
	var complete func(*hir.Method)
	complete = func(bridge *hir.Method) {
		if visited[bridge] {
			return
		}
		visited[bridge] = true
		target := l.bridgeTargets[bridge]
		if target == nil {
			return
		}
		complete(target)
		body := bridge.Body
		if body == nil || body.Kind != hir.Block || len(body.List) != 1 || body.List[0].Kind != hir.Return {
			return
		}
		ret := body.List[0]
		call := ret.X
		if call != nil && call.Kind == hir.Narrow {
			call = call.X
		}
		if call == nil || call.Kind != hir.VirtualCall {
			return
		}
		call.Name, call.Type = target.Name, target.Result
		ret.X = call
		src, dst := target.Result, bridge.Result
		if src.Kind == hir.Optional {
			src = src.Args[0]
		}
		if dst.Kind == hir.Optional {
			dst = dst.Args[0]
		}
		if src.Equal(dst) || (src.Kind != hir.ClassRef && src.Kind != hir.InterfaceRef) || (dst.Kind != hir.ClassRef && dst.Kind != hir.InterfaceRef) {
			return
		}
		// An erased object result needs a checked cast back to the slot's view.
		// Unrelated results remain mismatches; this is not a new cast relation.
		if !l.acceptsType(bridge.Result, target.Result) && l.acceptsType(src, dst) {
			ret.X = &hir.Expr{Kind: hir.Narrow, Node: bridge.Node, Type: bridge.Result, X: call}
		}
	}
	for bridge := range l.bridgeTargets {
		complete(bridge)
	}
}
