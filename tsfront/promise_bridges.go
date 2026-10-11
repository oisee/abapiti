package tsfront

import "github.com/oisee/abapiti/hir"

// A source async invocation always creates a fresh settled promise. Its erased
// virtual slot can therefore adopt that result with a covariant payload view,
// without exposing a second identity or changing a rejection into a throw.
// Ordinary promise-returning methods can return a shared promise; they cannot
// use this adaptation because rewrapping would change observable identity.
func (l *lowerer) completeAsyncBridges() {
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
		src, dst := target.Result, bridge.Result
		if !l.asyncMethods[target] || src.Kind != hir.Promise || dst.Kind != hir.Promise || src.Equal(dst) {
			return
		}
		from, to := src.Args[0], dst.Args[0]
		base := to
		if base.Kind == hir.Optional {
			base = base.Args[0]
		}
		if (base.Kind != hir.ClassRef && base.Kind != hir.InterfaceRef && base.Kind != hir.Array) || !l.acceptsType(to, from) {
			return
		}
		body := bridge.Body
		if body == nil || body.Kind != hir.Block || len(body.List) != 1 || body.List[0].Kind != hir.Return || body.List[0].X == nil {
			return
		}
		call := body.List[0].X
		value := &hir.Expr{Node: bridge.Node, Kind: hir.PromiseAwait, Type: from, X: call}
		view := &hir.Expr{Node: bridge.Node, Kind: hir.Narrow, Type: to, X: value}
		bridge.Body = &hir.Stmt{Node: bridge.Node, Kind: hir.Async, Body: hir.B(
			&hir.Stmt{Node: bridge.Node, Kind: hir.Return, X: &hir.Expr{Node: bridge.Node, Kind: hir.PromiseFulfill, Type: dst, X: view}},
		)}
		l.asyncMethods[bridge] = true
	}
	for bridge := range l.bridgeTargets {
		complete(bridge)
	}
}
