package rewrite

import "strconv"

// RuntimeEffects are adapter facts, independent of RuntimeSpec.Mutates (which
// describes the emitter's calling convention). Reads describe input dependencies;
// Args means any argument. Writes describe observable existing storage. Allocates
// includes strings, optional boxes and fresh containers, excluding allocator failure.
// Aliases is a conservative possible receiver alias, including splice1_view's
// shared backing storage. Element aliases are represented by ordinary flow facts.
// Reviewed against hir/runtime.go, hir/abap/{emit,runtime,dynamic_graph,
// json_subset,xml_subset,ordering}.go and the wasm runtime helpers/kernel.
// Trap includes fail-closed bounds/domain checks; Catchable includes type/tag and
// parse/regex failures. Conservative entries explain unresolved target differences.
type OpReads struct{ Receiver, Args, Global bool }
type OpEffect struct {
	Reads        OpReads
	Writes       string // "None", "Receiver", or "Arg(i)"
	Allocates    bool
	MayRaise     string // "None", "Trap", or "Catchable"
	Aliases      string // "None" or "Receiver"
	Conservative string
}

// Explicit records: adding a HIR catalogue operation requires a reviewed entry.
var RuntimeEffects = map[string]OpEffect{
	"array.concat":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.get":                 {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.includes":            {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"array.indexOf":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"array.join":                {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.length":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"array.pop":                 {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.push":                {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "None"},
	"array.reverse":             {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "Receiver"},
	"array.shift":               {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.slice0":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.slice1":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.slice2":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.splice1":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.splice1_view":        {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "Receiver"},
	"array.splice2":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.splice3":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "None"},
	"array.unshift":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "None"},
	"classvalue.has":            {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"classvalue.name":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"classvalue.new":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Dynamic constructor and initializer effects remain unknown until call targets are resolved."},
	"clock.telemetry":           {Reads: OpReads{Receiver: false, Args: false, Global: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.asBoolean":         {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.asClassValue":      {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.asNumber":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.asRef":             {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "Receiver"},
	"dynamic.asString":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.get":               {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.isArray":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.isFunction":        {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.isNullish":         {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.isNumber":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.isString":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.materialize":       {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Projection invokes a data-class constructor; its effects need separate call analysis."},
	"dynamic.null":              {Reads: OpReads{Receiver: false, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"dynamic.of":                {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"dynamic.put":               {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.strictEquals":      {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.toString":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.typeof":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"i32.toString":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"i64.remainder2":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"i64.toString":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"json.parseSubset":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"map.get":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"map.has":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"map.keys":                  {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"map.set":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "Receiver"},
	"map.size":                  {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"map.values":                {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"number.fromI32":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.index":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.remainder2":         {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.toString":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"object.classOf":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"record.delete":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "None"},
	"regexp.match_test":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None", Conservative: "Target regex engine may reject a supported-looking pattern at execution."},
	"regexp.new":                {Reads: OpReads{Receiver: false, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Pattern and flags validation differs between runtimes."},
	"regexp.source":             {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"regexp.test":               {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None", Conservative: "Target regex engine may reject a supported-looking pattern at execution."},
	"regexp.toString":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"set.add":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "Receiver"},
	"set.copy":                  {Reads: OpReads{Receiver: false, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"set.delete":                {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "None"},
	"set.fromArray":             {Reads: OpReads{Receiver: false, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"set.has":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"set.size":                  {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"set.values":                {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.at":                 {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.charAt":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.charCodeAt":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "None"},
	"string.compareObjectName":  {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"string.compareRegistryKey": {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"string.concat":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.endsWith":           {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.indexOf":            {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.length":             {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.localeCompareNames": {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "None"},
	"string.parseInt10":         {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Unbounded decimal accumulation may overflow the target numeric representation."},
	"string.parseInt10i64":      {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Decimal accumulation or conversion to int8 may overflow."},
	"string.repeatIndent":       {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"string.replaceAll":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "ABAP REPLACE ALL has no empty-needle guard; conservatively allow target replacement errors."},
	"string.replaceFirst":       {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.replaceRegex":       {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Unsupported patterns or replacement syntax may raise in a target runtime."},
	"string.slice":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.split":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.startsWith":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.substr":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.substring":          {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.toLowerCase":        {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.toUpperCase":        {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.trim":               {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"xml.parseSubset":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
}

func (x *extractor) addEffects() {
	for op, e := range RuntimeEffects {
		any := false
		for _, r := range []struct {
			name string
			yes  bool
		}{{"Receiver", e.Reads.Receiver}, {"Args", e.Reads.Args}, {"Global", e.Reads.Global}} {
			if r.yes {
				x.add("op_reads", op, r.name)
				any = true
			}
		}
		if !any {
			x.add("op_reads", op, "None")
		}
		x.add("op_writes", op, e.Writes)
		x.add("op_allocates", op, strconv.FormatBool(e.Allocates))
		x.add("op_may_raise", op, e.MayRaise)
		x.add("op_aliases", op, e.Aliases)
		if e.Conservative != "" {
			x.add("op_conservative", op, e.Conservative)
		}
	}
}
