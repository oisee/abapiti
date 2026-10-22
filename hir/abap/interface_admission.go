package abap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

type interfacePair struct{ class, iface string }

type interfaceAdmission struct {
	pair         interfacePair
	class        *hir.Class
	iface        *hir.Interface
	adapter      string
	node         hir.Node
	refusal      string
	dependencies []*interfaceAdmission
}

func interfaceBase(t hir.Type) hir.Type {
	if t.Kind == hir.Optional {
		return t.Args[0]
	}
	return t
}

// Collect only actual value flows, including the contextual conversions which
// do not have their own HIR Cast/Narrow node. Keep the shared HIR untouched.
func (e *emitter) collectInterfaceAdmissions() {
	var expr func(*hir.Expr, *hir.Class, *hir.Method)
	var stmt func(*hir.Stmt, *hir.Class, *hir.Method)
	flow := func(x *hir.Expr, dst hir.Type) {
		if x != nil {
			e.planInterfaceAdmission(x.Type, dst, x.Node)
		}
	}
	expr = func(x *hir.Expr, c *hir.Class, m *hir.Method) {
		if x == nil {
			return
		}
		switch x.Kind {
		case hir.Cast, hir.Narrow:
			e.planInterfaceAdmission(x.X.Type, x.Type, x.Node)
		case hir.Conditional:
			flow(x.Y, x.Type)
			flow(x.Z, x.Type)
		case hir.Seq:
			flow(x.Y, x.Type)
		case hir.New, hir.DirectCall, hir.VirtualCall, hir.SuperCall:
			var target *hir.Method
			owner := x.Owner
			if x.X != nil {
				owner = interfaceBase(x.X.Type).Name
			}
			if x.Kind == hir.SuperCall {
				owner = c.Super
			}
			if x.Kind == hir.New {
				target = e.p.Constructor(x.Type.Name)
			} else if x.Kind == hir.SuperCall && x.Name == "constructor" {
				target = e.p.Constructor(owner)
			} else {
				target, _ = e.method(e.classBy(owner), x.Name)
				if target == nil {
					for _, i := range e.p.Interfaces {
						if i.Name == owner {
							for _, im := range i.Methods {
								if im.Name == x.Name {
									target = im
								}
							}
						}
					}
				}
			}
			if target != nil {
				for j, a := range x.Args {
					if j < len(target.Params) {
						flow(a, target.Params[j].Type)
					}
				}
			}
		case hir.RuntimeOp:
			if x.X != nil {
				params, _, _ := hir.RuntimeSignature(x.Op, x.X.Type)
				for j, arg := range x.Args {
					if j < len(params) {
						flow(arg, params[j])
					}
				}
			}
		}
		expr(x.X, c, m)
		expr(x.Y, c, m)
		expr(x.Z, c, m)
		for _, a := range x.Args {
			expr(a, c, m)
		}
		stmt(x.Stmt, c, m)
	}
	stmt = func(s *hir.Stmt, c *hir.Class, m *hir.Method) {
		if s == nil {
			return
		}
		switch s.Kind {
		case hir.VarDecl:
			flow(s.X, s.Type)
		case hir.Assign:
			flow(s.Y, s.X.Type)
		case hir.Return:
			flow(s.X, m.Result)
		}
		expr(s.X, c, m)
		expr(s.Y, c, m)
		stmt(s.Body, c, m)
		stmt(s.Else, c, m)
		for _, child := range s.List {
			stmt(child, c, m)
		}
	}
	for _, c := range e.p.Classes {
		if c.Ctor != nil {
			stmt(c.Ctor.Body, c, c.Ctor)
		}
		for _, m := range c.Methods {
			stmt(m.Body, c, m)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, a := range e.interfaceAdmissionOrder {
			if a.refusal != "" {
				continue
			}
			for _, dependency := range a.dependencies {
				if dependency.refusal != "" {
					a.refusal = dependency.refusal
					changed = true
					break
				}
			}
		}
	}
}

func (e *emitter) planInterfaceAdmission(src, dst hir.Type, node hir.Node) *interfaceAdmission {
	src, dst = interfaceBase(src), interfaceBase(dst)
	if src.Kind == hir.Array && dst.Kind == hir.Array {
		return e.planInterfaceAdmission(src.Args[0], dst.Args[0], node)
	}
	if src.Kind != hir.ClassRef || dst.Kind != hir.InterfaceRef || e.upcast(src, dst) {
		return nil
	}
	pair := interfacePair{src.Name, dst.Name}
	if a := e.interfaceAdmissions[pair]; a != nil {
		return a
	}
	c := e.classBy(src.Name)
	if c == nil {
		return nil
	}
	var iface *hir.Interface
	for _, i := range e.p.Interfaces {
		if i.Name == dst.Name {
			iface = i
			break
		}
	}
	// Structural union admission has its own checked graph representation.
	if iface == nil || len(iface.StructuralMembers) != 0 {
		return nil
	}
	a := &interfaceAdmission{pair: pair, class: c, iface: iface, node: node}
	if e.interfaceAdmissions == nil {
		e.interfaceAdmissions = map[interfacePair]*interfaceAdmission{}
	}
	e.interfaceAdmissions[pair] = a
	e.interfaceAdmissionOrder = append(e.interfaceAdmissionOrder, a)
	for _, slot := range iface.Methods {
		impl, owner := e.method(c, slot.Name)
		if impl == nil || impl.Static || impl.Internal || e.fieldOwner(c, slot.Name) != nil || !e.interfaceSignature(slot, e.emittedMethod(owner, impl)) {
			a.adapter = e.name("view.interface." + c.Name + "." + iface.Name)
			break
		}
	}
	for _, slot := range iface.Methods {
		impl, _, _ := e.interfaceAdapterMember(c, slot.Name)
		if impl == nil {
			continue
		}
		if dependency := e.planInterfaceAdmission(impl.Result, slot.Result, node); dependency != nil {
			a.dependencies = append(a.dependencies, dependency)
		}
		for j, p := range impl.Params {
			if j < len(slot.Params) {
				if dependency := e.planInterfaceAdmission(slot.Params[j].Type, p.Type, node); dependency != nil {
					a.dependencies = append(a.dependencies, dependency)
				}
			}
		}
	}
	if a.adapter != "" {
		e.liveRuntime()
		// Delegating a differently typed result or argument can itself require
		// another structural admission. Plan those edges before declarations.
		for _, slot := range iface.Methods {
			impl, _, _ := e.interfaceAdapterMember(c, slot.Name)
			if impl == nil {
				if hidden, _ := e.method(c, slot.Name); hidden != nil && hidden.Internal {
					a.refusal = "private member " + c.Name + "." + slot.Name + " has no proven public callable layout"
				}
				for base := c; base != nil; base = e.classBy(base.Super) {
					if base.FieldsDropped {
						a.refusal = "unlowered fields on " + c.Name + " cannot prove callable property presence"
					}
				}
				if e.interfaceOpaqueMember(c, slot.Name) {
					a.refusal = "erased property " + c.Name + "." + slot.Name + " has no proven callable layout"
				}
				for _, child := range e.interfaceMemberCandidates(a, slot.Name) {
					member, _, _ := e.interfaceAdapterMember(child, slot.Name)
					e.planInterfaceDelegate(a, slot, member)
				}
				continue
			}
			e.planInterfaceDelegate(a, slot, impl)
		}
	}
	return a
}

func (e *emitter) planInterfaceDelegate(a *interfaceAdmission, slot, impl *hir.Method) {
	member := a.class.Name + "." + slot.Name
	if impl.Result.Kind == hir.Void && slot.Result.Kind != hir.Void && slot.Result.Kind != hir.Optional && !slot.Result.IsRef() {
		a.refusal = "void result of " + member + " cannot retain undefined"
	}
	for j, p := range impl.Params {
		if j < len(slot.Params) {
			if p.Variadic != slot.Params[j].Variadic {
				a.refusal = "rest parameter of " + member + " has a different calling convention"
			}
			if dependency := e.planInterfaceAdmission(slot.Params[j].Type, p.Type, a.node); dependency != nil {
				a.dependencies = append(a.dependencies, dependency)
			}
			if !e.interfaceConvertible(slot.Params[j].Type, p.Type) {
				a.refusal = "parameter conversion of " + member + " cannot preserve the source value"
			}
		} else if p.Type.Kind != hir.Optional {
			a.refusal = "missing argument of " + member + " cannot retain undefined"
		}
	}
	if dependency := e.planInterfaceAdmission(impl.Result, slot.Result, a.node); dependency != nil {
		a.dependencies = append(a.dependencies, dependency)
	}
	if impl.Result.Kind != hir.Void && slot.Result.Kind != hir.Void && !e.interfaceConvertible(impl.Result, slot.Result) {
		a.refusal = "result conversion of " + member + " cannot preserve the source value"
	}
}

func (e *emitter) interfaceMemberCandidates(a *interfaceAdmission, name string) []*hir.Class {
	var candidates []*hir.Class
	for _, c := range e.descendants(a.class) {
		if e.upcast(hir.Ref(c.Name), hir.Type{Kind: hir.InterfaceRef, Name: a.iface.Name}) {
			continue
		}
		if m, _, _ := e.interfaceAdapterMember(c, name); m != nil {
			candidates = append(candidates, c)
		}
	}
	depth := func(c *hir.Class) int {
		n := 0
		for ; c != nil; c = e.classBy(c.Super) {
			n++
		}
		return n
	}
	sort.SliceStable(candidates, func(j, k int) bool { return depth(candidates[j]) > depth(candidates[k]) })
	return candidates
}

func (e *emitter) interfaceSignature(slot, impl *hir.Method) bool {
	if len(slot.Params) != len(impl.Params) || slot.Result.Kind != hir.Void && !e.interfaceABI(slot.Result).Equal(e.interfaceABI(impl.Result)) {
		return false
	}
	for j, p := range slot.Params {
		if p.Variadic != impl.Params[j].Variadic || !e.interfaceABI(p.Type).Equal(e.interfaceABI(impl.Params[j].Type)) {
			return false
		}
	}
	return true
}

// Comparing signatures must not instantiate a container runtime before all
// adapter/identity gates are known. These are typ's reference storage rules.
func (e *emitter) interfaceABI(t hir.Type) hir.Type {
	t = e.containerStorage(arrayStorage(unionStorage(t)))
	if t.Kind == hir.Optional && t.Args[0].IsRef() {
		return e.interfaceABI(t.Args[0])
	}
	return t
}

func (e *emitter) inheritedAdmission(c *hir.Class, iface string) bool {
	for base := e.classBy(c.Super); base != nil; base = e.classBy(base.Super) {
		if a := e.interfaceAdmissions[interfacePair{base.Name, iface}]; a != nil && a.adapter == "" && a.refusal == "" {
			return true
		}
	}
	return false
}

func (e *emitter) structuralInterfaceDeclarations(out *strings.Builder, c *hir.Class) {
	for _, a := range e.interfaceAdmissionOrder {
		if a.class.Name == c.Name && a.adapter == "" && a.refusal == "" && !e.inheritedAdmission(c, a.iface.Name) {
			fmt.Fprintf(out, "INTERFACES %s.\n", e.name(a.iface.Name))
		}
	}
}

func (e *emitter) structuralInterfaceImplementations(out *strings.Builder, c *hir.Class) {
	for _, a := range e.interfaceAdmissionOrder {
		if a.class.Name != c.Name || a.adapter != "" || a.refusal != "" || e.inheritedAdmission(c, a.iface.Name) {
			continue
		}
		if e.descriptors {
			fmt.Fprintf(out, "METHOD %s~%s.\nresult = me->%s( ).\nENDMETHOD.\n", e.name(a.iface.Name), e.name("builtin.classOf"), e.name("builtin.classOf"))
		}
		for _, slot := range a.iface.Methods {
			impl, _ := e.method(c, slot.Name)
			fmt.Fprintf(out, "METHOD %s~%s.\n", e.name(a.iface.Name), e.member(slot.Name))
			args := []string{}
			for j, p := range impl.Params {
				args = append(args, e.param(p.Name)+" = "+e.param(slot.Params[j].Name))
			}
			if slot.Result.Kind != hir.Void {
				out.WriteString("result = ")
			}
			fmt.Fprintf(out, "me->%s( %s ).\nENDMETHOD.\n", e.member(impl.Name), strings.Join(args, " "))
		}
	}
}

func (b *body) admitInterface(value string, src, dst hir.Type) (string, bool) {
	src, dst = interfaceBase(src), interfaceBase(dst)
	if src.Kind == hir.Array && dst.Kind == hir.Array {
		if a := b.e.interfaceFlowAdmission(src, dst); a != nil && a.refusal != "" {
			b.e.refuse(hir.Error{Node: a.node, Message: "ABAP structural interface " + a.iface.Name + ": " + a.refusal})
			return value, true
		}
		return "", false
	}
	if src.Kind != hir.ClassRef || dst.Kind != hir.InterfaceRef {
		return "", false
	}
	a := b.e.interfaceAdmissions[interfacePair{src.Name, dst.Name}]
	if a == nil {
		return "", false
	}
	if a.refusal != "" {
		b.e.refuse(hir.Error{Node: a.node, Message: "ABAP structural interface " + a.iface.Name + ": " + a.refusal})
		return value, true
	}
	if a.adapter == "" {
		return value, true
	}
	n := b.temp(dst)
	b.line(n + " = " + b.e.interfaceFactory(dst) + "=>convert( " + value + " ).")
	return n, true
}

func (e *emitter) interfaceFlowAdmission(src, dst hir.Type) *interfaceAdmission {
	src, dst = interfaceBase(src), interfaceBase(dst)
	if src.Kind == hir.Array && dst.Kind == hir.Array {
		return e.interfaceFlowAdmission(src.Args[0], dst.Args[0])
	}
	if src.Kind == hir.ClassRef && dst.Kind == hir.InterfaceRef {
		return e.interfaceAdmissions[interfacePair{src.Name, dst.Name}]
	}
	return nil
}

func (e *emitter) emitInterfaceAdapters() {
	factories := map[string]bool{}
	for _, a := range e.interfaceAdmissionOrder {
		if a.adapter == "" || a.refusal != "" {
			continue
		}
		e.emitInterfaceAdapter(a)
		if !factories[a.iface.Name] {
			factories[a.iface.Name] = true
			e.emitInterfaceFactory(a.iface)
		}
	}
}

func (e *emitter) interfaceFactory(t hir.Type) string {
	if t.Kind != hir.InterfaceRef {
		return ""
	}
	for _, a := range e.interfaceAdmissionOrder {
		if a.iface.Name == t.Name && a.adapter != "" && a.refusal == "" {
			return e.name("view.interfaceFactory." + t.Name)
		}
	}
	return ""
}

func (e *emitter) hasInterfaceAdapters() bool {
	for _, a := range e.interfaceAdmissionOrder {
		if a.adapter != "" && a.refusal == "" {
			return true
		}
	}
	return false
}

func (e *emitter) emitInterfaceFactory(i *hir.Interface) {
	t := hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
	name := e.interfaceFactory(t)
	var out strings.Builder
	fmt.Fprintf(&out, "CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS convert IMPORTING p0 TYPE REF TO object RETURNING VALUE(result) TYPE REF TO %s.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD convert.\nIF p0 IS NOT BOUND.\nRETURN.\nENDIF.\nIF p0 IS INSTANCE OF %s.\nresult ?= p0.\nRETURN.\nENDIF.\n", name, e.name(i.Name), name, e.name(i.Name))
	var candidates []*interfaceAdmission
	for _, a := range e.interfaceAdmissionOrder {
		if a.iface == i && a.adapter != "" && a.refusal == "" {
			candidates = append(candidates, a)
		}
	}
	depth := func(c *hir.Class) int {
		n := 0
		for ; c != nil; c = e.classBy(c.Super) {
			n++
		}
		return n
	}
	sort.SliceStable(candidates, func(j, k int) bool { return depth(candidates[j].class) > depth(candidates[k].class) })
	for j, a := range candidates {
		fmt.Fprintf(&out, "IF p0 IS INSTANCE OF %s.\nDATA source%d TYPE REF TO %s.\nsource%d ?= p0.\nresult = NEW %s( source = source%d ).\nRETURN.\nENDIF.\n", e.name(a.class.Name), j, e.name(a.class.Name), j, a.adapter, j)
	}
	out.WriteString("result ?= p0.\nENDMETHOD.\nENDCLASS.\n")
	e.files[name+".clas.abap"] = out.String()
}

// Erasing an adapter exposes the original reference. Recover an interface
// through the same used-pair factory, including after any/unknown round trips.
func (b *body) recoverInterface(n, raw string, t hir.Type) bool {
	if factory := b.e.interfaceFactory(interfaceBase(t)); factory != "" {
		b.line(n + " = " + factory + "=>convert( " + raw + " ).")
		return true
	}
	return false
}

func (b *body) interfaceOriginal(value string, t hir.Type) string {
	if b.e.interfaceFactory(interfaceBase(t)) == "" {
		return value
	}
	boxed, raw := b.temp(hir.T(hir.Dynamic)), b.temp(hir.Ref(hir.RootObject))
	b.boxDynamic(boxed, value, t)
	b.line("CLEAR " + raw + ".\nIF " + boxed + " IS BOUND.\n" + raw + " = " + boxed + "->as_ref( ).\nENDIF.")
	return raw
}

func (e *emitter) interfaceArrayIdentity(t hir.Type, method, code string) string {
	if !e.hasInterfaceAdapters() || !t.Args[0].IsRef() {
		return code
	}
	if method == "includes" {
		return "result = xsdbool( indexOf( p0 ) >= 0 ).\n"
	}
	if method != "indexOf" {
		return code
	}
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.I32)}, locals: map[string]string{}}
	row, left, right := b.temp(t.Args[0]), b.temp(hir.T(hir.Dynamic)), b.temp(hir.T(hir.Dynamic))
	b.line("result = -1.")
	b.boxDynamic(right, "p0", t.Args[0])
	b.line("LOOP AT items INTO " + row + ".")
	index := b.temp(hir.T(hir.I32))
	b.line(index + " = sy-tabix - 1.")
	b.boxDynamic(left, row, t.Args[0])
	b.line("IF " + left + " IS NOT BOUND.\nIF " + right + " IS NOT BOUND.\nresult = " + index + ".\nRETURN.\nENDIF.\nELSEIF " + left + "->strict_equals( " + right + " ) = abap_true.\nresult = " + index + ".\nRETURN.\nENDIF.\nENDLOOP.")
	return b.code.String()
}

func (e *emitter) resolveInterfaceArrayIdentity() {
	if !e.hasInterfaceAdapters() {
		return
	}
	for _, t := range e.chaiArrays {
		if !t.Args[0].IsRef() {
			continue
		}
		file := e.name("runtime."+t.String()) + ".clas.abap"
		text := e.files[file]
		for _, method := range []string{"indexOf", "includes"} {
			start := strings.Index(text, "METHOD "+method+".\n")
			if start < 0 {
				continue
			}
			end := start + strings.Index(text[start:], "ENDMETHOD.\n") + len("ENDMETHOD.\n")
			code := "dense( ).\n" + e.interfaceArrayIdentity(t, method, "")
			if e.arrayViews {
				code = "load( ).\n" + code
			}
			text = text[:start] + "METHOD " + method + ".\n" + code + "ENDMETHOD.\n" + text[end:]
		}
		e.files[file] = text
	}
}

// Native reference tables retain their key, so canonicalizing a wrapper at
// insertion would lose its interface on iteration. The reviewed container-view
// ABI handles that case; otherwise refuse at the observed collection operation.
func (b *body) refuseInterfaceKeyIdentity(x *hir.Expr) bool {
	if !b.e.hasInterfaceAdapters() || x.X == nil {
		return false
	}
	t := x.X.Type
	if t.Kind != hir.OrderedMap && t.Kind != hir.OrderedSet || len(t.Args) == 0 {
		return false
	}
	key := interfaceBase(t.Args[0])
	if b.e.interfaceFactory(key) == "" {
		return false
	}
	switch hir.CheckedMapBaseOp(x.Op) {
	case "map.set", "map.get", "map.has", "map.delete", "set.add", "set.has", "set.delete":
	default:
		return false
	}
	if t.Kind == hir.OrderedMap && b.e.typedMapViews() || t.Kind == hir.OrderedSet && b.e.usesReviewedOp("set.view") {
		return false
	}
	b.e.refuse(hir.Error{Node: x.Node, Message: "ABAP structural interface adapters require canonical identity for reference collection keys (" + x.Op + ")"})
	return true
}

func (e *emitter) emitInterfaceAdapter(a *interfaceAdmission) {
	var out strings.Builder
	i, storage := e.name(a.iface.Name), e.name("view.storage")
	fmt.Fprintf(&out, "CLASS %s DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nINTERFACES %s.\nINTERFACES %s.\nMETHODS constructor IMPORTING source TYPE REF TO %s.\nPROTECTED SECTION.\nPRIVATE SECTION.\nDATA source TYPE REF TO %s.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD constructor.\nme->source = source.\nENDMETHOD.\n", a.adapter, i, storage, e.name(a.class.Name), e.name(a.class.Name), a.adapter)
	if e.descriptors {
		fmt.Fprintf(&out, "METHOD %s~%s.\nresult = source->%s( ).\nENDMETHOD.\n", i, e.name("builtin.classOf"), e.name("builtin.classOf"))
	}
	for _, slot := range a.iface.Methods {
		b := &body{e: e, c: a.class, m: slot, locals: map[string]string{}}
		impl, _, _ := e.interfaceAdapterMember(a.class, slot.Name)
		if impl == nil {
			for _, child := range e.interfaceMemberCandidates(a, slot.Name) {
				b.line("IF source IS INSTANCE OF " + e.name(child.Name) + ".")
				source := b.temp(hir.Ref(child.Name))
				b.line(source + " ?= source.")
				b.interfaceDelegate(child, slot, source)
				b.line("RETURN.\nENDIF.")
			}
			if getter, _ := e.method(a.class, "get."+slot.Name); getter != nil && !getter.Static && !getter.Internal {
				b.locals["source"] = "source"
				b.expr(&hir.Expr{Kind: hir.VirtualCall, Type: getter.Result, Name: getter.Name, X: hir.V("source", hir.Ref(a.class.Name))})
			}
			// Missing optional members remain absent on the original object.
			// Calling an absent/noncallable property is a source TypeError.
			b.interfaceTypeError(slot.Name)
			fmt.Fprintf(&out, "METHOD %s~%s.\n%s%sENDMETHOD.\n", i, e.member(slot.Name), b.constantDeclarations(), b.code.String())
			continue
		}
		b.interfaceDelegate(a.class, slot, "source")
		fmt.Fprintf(&out, "METHOD %s~%s.\n%s%sENDMETHOD.\n", i, e.member(slot.Name), b.constantDeclarations(), b.code.String())
	}
	b := &body{e: e, c: a.class, m: &hir.Method{Result: hir.T(hir.Dynamic)}, locals: map[string]string{}}
	b.boxDynamic("result", "source", hir.Ref(a.class.Name))
	fmt.Fprintf(&out, "METHOD %s~source.\n%s%sENDMETHOD.\nMETHOD %s~structural.\nENDMETHOD.\n", storage, b.constantDeclarations(), b.code.String(), storage)
	for _, name := range []string{"get", "put", "has", "delete"} {
		fmt.Fprintf(&out, "METHOD %s~%s.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `interface adapter has no erased property layout`.\nENDMETHOD.\n", storage, name, e.name("view.refusal"))
	}
	out.WriteString("ENDCLASS.\n")
	e.files[a.adapter+".clas.abap"] = out.String()
}

func (b *body) interfaceDelegate(c *hir.Class, slot *hir.Method, source string) {
	e := b.e
	impl, owner, receiver := e.interfaceAdapterMember(c, slot.Name)
	if owner != nil {
		impl = e.emittedMethod(owner, impl)
	}
	recv := source
	if receiver != nil {
		b.locals["source"] = source
		recv = b.expr(receiver)
		b.line("IF " + recv + " IS NOT BOUND.")
		b.interfaceTypeError(slot.Name)
		b.line("ENDIF.")
	}
	args := []string{}
	for j, p := range impl.Params {
		if j >= len(slot.Params) {
			args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
			continue
		}
		args = append(args, e.param(p.Name)+" = "+b.interfaceConvert(e.param(slot.Params[j].Name), slot.Params[j].Type, p.Type))
	}
	call := recv + "->" + e.member(impl.Name) + "( " + strings.Join(args, " ") + " )"
	if slot.Result.Kind == hir.Void {
		b.line(call + ".")
	} else if impl.Result.Kind == hir.Void {
		b.line(call + ".\nCLEAR result.")
	} else {
		value := b.temp(impl.Result)
		b.line(value + " = " + call + ".")
		b.line("result = " + b.interfaceConvert(value, impl.Result, slot.Result) + ".")
	}
}

func (b *body) interfaceConvert(value string, src, dst hir.Type) string {
	// Optional any/unknown uses the same ABAP reference as Dynamic. Keep
	// absence through reference and optional result slots before unboxing.
	if interfaceBase(src).Kind == hir.Dynamic {
		src = hir.T(hir.Dynamic)
		if dst.Kind == hir.Optional && !dst.Args[0].IsRef() {
			n := b.temp(dst)
			b.liveUnbox(n, value, dst)
			return n
		}
		if referenceStorage(dst) && interfaceBase(dst).Kind != hir.Dynamic {
			n := b.temp(dst)
			b.line("CLEAR " + n + ".\nIF " + value + " IS BOUND.")
			b.line(n + " = " + b.convert(value, src, interfaceBase(dst)) + ".\nENDIF.")
			return n
		}
	}
	if interfaceBase(dst).Kind == hir.Dynamic {
		dst = hir.T(hir.Dynamic)
	}
	return b.convert(value, src, dst)
}

func (e *emitter) interfaceConvertible(src, dst hir.Type) bool {
	return e.interfaceABI(src).Equal(e.interfaceABI(dst)) || dst.Kind == hir.Dynamic || src.Kind == hir.Dynamic || referenceStorage(src) && referenceStorage(dst) || dst.Kind == hir.Optional && src.Equal(dst.Args[0])
}

// Callable data properties use their function object's call slot. A scalar
// property shadowing a method is noncallable, just like a missing member.
func (e *emitter) interfaceAdapterMember(c *hir.Class, name string) (*hir.Method, *hir.Class, *hir.Expr) {
	if owner := e.fieldOwner(c, name); owner != nil {
		for _, f := range owner.Fields {
			if f.Name != name || f.Static || f.Private {
				continue
			}
			call := e.interfaceCallable(f.Type)
			if call != nil {
				return call, nil, &hir.Expr{Kind: hir.FieldGet, Type: f.Type, X: hir.V("source", hir.Ref(c.Name)), Name: name}
			}
			return nil, nil, nil
		}
	}
	if getter, _ := e.method(c, "get."+name); getter != nil && !getter.Static && !getter.Internal {
		if call := e.interfaceCallable(getter.Result); call != nil {
			return call, nil, &hir.Expr{Kind: hir.VirtualCall, Type: getter.Result, Name: getter.Name, X: hir.V("source", hir.Ref(c.Name))}
		}
		return nil, nil, nil
	}
	m, owner := e.method(c, name)
	if m == nil || m.Static || m.Internal {
		return nil, nil, nil
	}
	return m, owner, nil
}

func (e *emitter) interfaceCallable(t hir.Type) *hir.Method {
	t = interfaceBase(t)
	if t.Kind == hir.InterfaceRef {
		for _, i := range e.p.Interfaces {
			if i.Name == t.Name {
				for _, m := range i.Methods {
					if m.Name == "call" {
						return m
					}
				}
			}
		}
	} else if t.Kind == hir.ClassRef {
		m, _ := e.method(e.classBy(t.Name), "call")
		return m
	}
	return nil
}

func (e *emitter) interfaceOpaqueMember(c *hir.Class, name string) bool {
	if owner := e.fieldOwner(c, name); owner != nil {
		for _, f := range owner.Fields {
			if f.Name == name {
				t := interfaceBase(f.Type)
				return t.Kind == hir.Dynamic || t.Kind == hir.UnionRef || f.Private
			}
		}
	}
	if getter, _ := e.method(c, "get."+name); getter != nil {
		t := interfaceBase(getter.Result)
		return t.Kind == hir.Dynamic || t.Kind == hir.UnionRef || getter.Internal
	}
	return false
}

func (b *body) interfaceTypeError(member string) {
	e := b.e
	e.errorUsed = true
	t := hir.Ref("builtin.TypeError")
	cls := e.name(t.Name)
	if e.classBy(t.Name) == nil {
		e.files[cls+".clas.abap"] = "CLASS " + cls + " DEFINITION PUBLIC INHERITING FROM " + e.name("builtin.Error") + " CREATE PUBLIC.\nPUBLIC SECTION.\nENDCLASS.\nCLASS " + cls + " IMPLEMENTATION.\nENDCLASS.\n"
	}
	message := b.temp(hir.T(hir.Optional, hir.T(hir.String)))
	b.line(message + " = NEW #( ).\n" + message + "->has = abap_true.\n" + message + "->value = `" + strings.ReplaceAll(member, "`", "``") + " is not a function`.")
	value := b.temp(t)
	b.line(value + " = NEW " + cls + "( " + e.param("message") + " = " + message + " ).")
	wrapper := e.exception(t)
	if e.classBy(t.Name) == nil {
		e.files[wrapper+".clas.abap"] = "CLASS " + wrapper + " DEFINITION PUBLIC INHERITING FROM " + e.exception(hir.Ref("builtin.Error")) + " CREATE PUBLIC.\nPUBLIC SECTION.\nENDCLASS.\nCLASS " + wrapper + " IMPLEMENTATION.\nENDCLASS.\n"
	}
	err := b.rawTemp("REF TO " + wrapper)
	b.line(err + " = NEW #( ).\n" + err + "->payload = " + value + ".")
	if e.promiseUsed {
		b.promisePayload(err+"->source_value", value, t)
	}
	b.line("RAISE EXCEPTION " + err + ".")
}
