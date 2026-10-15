package rewrite

import (
	"strconv"
	"strings"
)

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
	Writes       string // "None", "Receiver", "Arg(i)", or "Global" (unknown callback effects)
	Allocates    bool
	MayRaise     string // "None", "Trap", or "Catchable"
	Aliases      string // "None" or "Receiver"
	Conservative string
}

// Explicit records: adding a HIR catalogue operation requires a reviewed entry.
var RuntimeEffects = map[string]OpEffect{
	"union.admit":               {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "Receiver"},
	"union.widen":               {Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "Catchable", Aliases: "Receiver"},
	"union.project":             {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "Receiver"},
	"union.tag":                 {Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "Catchable", Aliases: "None"},
	"union.read":                {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "Receiver"},
	"string.sha1Hex":            {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Fresh UTF-8 SHA-1 hex pipeline; Go is pure, ABAP explicitly refuses."},
	"array.has":                 {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", MayRaise: "None", Aliases: "None"},
	"array.delete":              {Reads: OpReads{Receiver: true, Args: true}, Writes: "Receiver", Allocates: true, MayRaise: "None", Aliases: "None"},
	"json.stringifySubset":      {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "JSON data graph; unsupported native order and hooks refuse; ABAP refuses."},
	"json.stringifyConfig":      {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Source-pinned config graph writer."},
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
	"array.spread0":             {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
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
	"classvalue.newArgs":        {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Dynamic constructor invocation may run arbitrary constructor effects."},
	"classvalue.new":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Dynamic constructor and initializer effects remain unknown until call targets are resolved."},
	"clock.telemetry":           {Reads: OpReads{Receiver: false, Args: false, Global: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.asBoolean":         {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.asClassValue":      {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"dynamic.asNumber":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"lsp.isTextDocumentEdit":    {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "None"},
	"lsp.isCreateFile":          {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "None"},
	"lsp.isDeleteFile":          {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "None"},
	"lsp.isRenameFile":          {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "None"},
	"dynamic.keys":              {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"dynamic.spread":            {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"dynamic.isMap":             {Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "None", Aliases: "None"},
	"dynamic.isSet":             {Reads: OpReads{Receiver: true}, Writes: "None", MayRaise: "None", Aliases: "None"},
	"dynamic.classView":         {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "Receiver", Conservative: "A class view aliases structural storage; unsupported prototype behaviour refuses."},
	"dynamic.arrayView":         {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "Receiver", Conservative: "A typed array view aliases original storage and checks each element read."},
	"dynamic.asRef":             {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "Receiver"},
	"dynamic.asString":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"object.get":                {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "Receiver"},
	"object.hasOwn":             {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", MayRaise: "Catchable", Aliases: "None"},
	"object.has":                {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", MayRaise: "Catchable", Aliases: "None"},
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
	"chai.throwMatch":           {Reads: OpReads{Receiver: true, Args: true, Global: true}, Writes: "Global", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Tests-only invokes certified parameterless callable storage, ignoring synchronous results; callback effects unknown, host faults escape, only string/Error throws match; ABAP refuses."},
	"chai.members":              {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"chai.arrayInclude":         {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Tests-only Chai strict array inclusion; nonarrays and snapshot escapes refuse; ABAP refuses."},
	"chai.box":                  {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Tests-only boxing; parsed/native snapshot escapes refuse; ABAP refuses."},
	"chai.deepEqual":            {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Tests-only Chai data graph; unproven enumerable storage refuses; ABAP refuses."},
	"chai.deepInclude":          {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None", Conservative: "Tests-only Chai subset assertion; unsupported property/type domains refuse; ABAP refuses."},
	"dynamic.strictEquals":      {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.recordString":      {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"dynamic.toString":          {Reads: OpReads{Receiver: true, Global: true}, Writes: "Global", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Reviewed native toString hooks may invoke arbitrary source effects; other ToPrimitive hooks refuse."},
	"dynamic.call0":             {Reads: OpReads{Receiver: true, Args: true, Global: true}, Writes: "Global", Allocates: true, MayRaise: "Catchable", Aliases: "Receiver", Conservative: "Invokes a known concrete native zero-argument method; source effects and result aliases are conservative; nonnative receivers refuse; ABAP refuses."},
	"dynamic.typeof":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"dynamic.typeofValue":       {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"i32.toString":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"i64.remainder2":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"i64.toString":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"json.indent":               {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"json.parseSubset":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"json5.parse":               {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"map.get":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"map.has":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"map.keys":                  {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"map.set":                   {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "Receiver"},
	"map.size":                  {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"map.values":                {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"number.fromI32":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.index":              {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"object.checkedClassView":   {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "Trap", Aliases: "Receiver", Conservative: "Unproven nominal class assertions keep identity only in the checked runtime domain; ABAP refuses."},
	"number.ceil":               {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.remainder2":         {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.toString":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"object.classOf":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"map.delete":                {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "None"},
	"dynamic.delete":            {Reads: OpReads{Receiver: true, Args: true}, Writes: "Receiver", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"record.delete":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "None", Aliases: "None"},
	"regexp.match_test":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "Catchable", Aliases: "None", Conservative: "Target regex engine may reject a supported-looking pattern at execution."},
	"regexp.new":                {Reads: OpReads{Receiver: false, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Pattern and flags validation differs between runtimes."},
	"regexp.source":             {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"regexp.lastIndex":          {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"regexp.setLastIndex":       {Reads: OpReads{Receiver: true, Args: true}, Writes: "Receiver", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"regexp.exec":               {Reads: OpReads{Receiver: true, Args: true}, Writes: "Receiver", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"regexp.test":               {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "Receiver", Allocates: false, MayRaise: "Catchable", Aliases: "None", Conservative: "Target regex engine may reject a supported-looking pattern at execution."},
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
	"string.parseFloatNumber":   {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.parseIntNumber":     {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"number.isNaN":              {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.parseFloat":         {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.padStart":           {Reads: OpReads{Receiver: true, Args: true}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"string.repeat":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"string.replaceAll":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "ABAP REPLACE ALL has no empty-needle guard; conservatively allow target replacement errors."},
	"string.replaceFirst":       {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.replaceRegex":       {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None", Conservative: "Unsupported patterns or replacement syntax may raise in a target runtime."},
	"string.slice":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.match":              {Reads: OpReads{Receiver: true, Args: true}, Writes: "Arg(0)", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"array.matchIndex":          {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: false, MayRaise: "Catchable", Aliases: "None"},
	"string.splitRegex":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "Trap", Aliases: "None"},
	"string.split":              {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.startsWith":         {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: false, MayRaise: "None", Aliases: "None"},
	"string.substr":             {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.substring":          {Reads: OpReads{Receiver: true, Args: true, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.toLowerCase":        {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.toUpperCase":        {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.trimStart":          {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.trimEnd":            {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"string.trim":               {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"xml.validate":              {Reads: OpReads{Receiver: true}, Writes: "None", Allocates: true, MayRaise: "None", Aliases: "None"},
	"xml.parseSubset":           {Reads: OpReads{Receiver: true, Args: false, Global: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
	"xml.parseWithErrors":       {Reads: OpReads{Receiver: true, Args: false}, Writes: "None", Allocates: true, MayRaise: "Catchable", Aliases: "None"},
}

func (x *extractor) addEffects(includeUnions bool) {
	for op, e := range RuntimeEffects {
		// Keep unrelated source-pinned facts stable. A verified union operation
		// always has a family declaration in the same program.
		if strings.HasPrefix(op, "union.") && !includeUnions {
			continue
		}
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
