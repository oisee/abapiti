package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// EmitAbaplintTestsOSG emits the original test registration closure and an ABAP
// runner. Target refusals are traps, never assertions. Unimplemented hook/async
// execution is explicitly refused, not silently skipped or credited as a pass.
func EmitAbaplintTestsOSG(dir string, lowering *RegistryLowering) (map[string]string, error) {
	return emitAbaplintTestsOSG(dir, lowering, nil)
}

// EmitAbaplintTestsCore emits the full test closure and a native zabapgit
// client using precisely the same names and translated core.
func EmitAbaplintTestsCore(dir string, lowering *RegistryLowering, wantSHA string) (map[string]string, string, error) {
	var report, driver string
	files, err := emitAbaplintTestsOSG(dir, lowering, func(names *hir.Names) {
		report = RegistryCLIReport("zabaplint", names)
		driver = RegistryRunCorpusClass("zcl_lnt_registry_a4h", wantSHA, nil, names)
	})
	if err == nil {
		files["zcl_lnt_registry_a4h.clas.abap"] = driver
		files["zlnt_registry_run.prog.abap"] = RegistryRunReport("zlnt_registry_run", "zcl_lnt_registry_a4h", false)
		files["zlnt_registry_clean.prog.abap"] = RegistryRunReport("zlnt_registry_clean", "zcl_lnt_registry_a4h", false)
	}
	return files, report, err
}

func emitAbaplintTestsOSG(dir string, lowering *RegistryLowering, client func(*hir.Names)) (map[string]string, error) {
	if err := lowering.Err(); err != nil {
		return nil, err
	}
	hir.AssignSiteIDs(lowering.Prog)
	if err := TrapABAPTestRefusals(lowering); err != nil {
		return nil, err
	}
	assertionOwners := map[string]bool{"runtime/chai.ts.Assertion.check": true}
	for _, c := range lowering.Prog.Classes {
		if c.Name == "runtime/chai.ts.module" {
			for _, m := range c.Methods {
				if strings.Contains(m.Name, "chaiFail") {
					assertionOwners[c.Name+"."+m.Name] = true
				}
			}
		}
	}
	emit := abap.EmitTestNamed
	if client != nil {
		emit = abap.EmitTestPackageNamed
	}
	files, names, err := emit(lowering.Prog, assertionOwners)
	if err != nil {
		return nil, err
	}
	if client != nil {
		client(names)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Initialization []string `json:"initialization"`
		Shards         []struct {
			Name           string   `json:"name"`
			Initialization []string `json:"initialization"`
		} `json:"shards"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	if len(manifest.Initialization) == 0 {
		return nil, fmt.Errorf("test closure has no initialization order")
	}
	modules := map[string]*hir.Class{}
	for _, c := range lowering.Prog.Classes {
		modules[c.Name] = c
	}
	if modules["runtime/mocha.ts.TestRuntime"] == nil {
		return nil, fmt.Errorf("test closure has no TestRuntime")
	}
	n := names.Get
	assertionClass := n("exception.testAssertion." + hir.Ref("builtin.Error").String())
	var initialization, hostDeclarations, hostMethods strings.Builder
	initIndex, hostIndex := 0, 0
	addAction := func(class, file, action string) {
		initIndex++
		hostIndex++
		boundary := fmt.Sprintf("host_init_%d", hostIndex)
		fmt.Fprintf(&hostDeclarations, "CLASS-METHODS %s RETURNING VALUE(result) TYPE string.\n", boundary)
		fmt.Fprintf(&hostMethods, "METHOD %s.\n%s=>%s( ).\nENDMETHOD.\n", boundary, n(class), action)
		fmt.Fprintf(&initialization, "IF skip_init NS ',%d,'.\ncheckpoint( `{\"kind\":\"init\",\"index\":%d,\"file\":` && quote( %s ) && `,\"action\":` && quote( %s ) && `}` ).\n", initIndex, initIndex, abapTestLiteral(file), abapTestLiteral(action))
		fmt.Fprintf(&initialization, "TRY.\nDATA(host_message_%d) = %s( ).\nIF host_message_%d IS NOT INITIAL.\nadd_initialization( file = %s action = %s status = 'trapped' message = host_message_%d ).\nENDIF.\nCATCH %s INTO init_assertion.\nadd_initialization( file = %s action = %s status = 'failed' message = init_assertion->payload->%s ).\nCATCH %s INTO init_trap.\nadd_initialization( file = %s action = %s status = 'trapped' message = init_trap->source_location ).\nCATCH cx_root INTO init_error.\nerror_class = cl_abap_classdescr=>get_class_name( init_error ).\nREPLACE FIRST OCCURRENCE OF '\\CLASS=' IN error_class WITH ''.\nerror_text = init_error->get_text( ).\nadd_initialization( file = %s action = %s status = 'trapped' message = error_class && |: | && error_text ).\nENDTRY.\n", hostIndex, boundary, hostIndex, abapTestLiteral(file), abapTestLiteral(action), hostIndex, assertionClass, abapTestLiteral(file), abapTestLiteral(action), n("member.message"), n("exception.unexecuted"), abapTestLiteral(file), abapTestLiteral(action), abapTestLiteral(file), abapTestLiteral(action))
		initialization.WriteString("checkpoint( `{\"kind\":\"init-done\"}` ).\nENDIF.\n")
	}
	// Each shard replays exactly its own dependency-first manifest, including
	// borrowed test modules. Local checkpoint indices restart at one per shard.
	runtime := modules["runtime/mocha.ts.TestRuntime"]
	addOrder := func(order []string) {
		for _, a := range ModuleInitializations(modules, order, true) {
			addAction(a.Class, a.File, n(a.Name))
		}
		// Force the runtime initializer even when a closure did not otherwise use it.
		for _, m := range runtime.Methods {
			if m.Static && m.Name == "class_constructor" {
				addAction(runtime.Name, "runtime/mocha.ts", n("builtin.initialize."+runtime.Name))
			}
		}
	}
	var selectedInitialization strings.Builder
	if len(manifest.Shards) == 0 {
		addOrder(manifest.Initialization)
		fmt.Fprintf(&selectedInitialization, "checkpoint( '{\"kind\":\"plan\",\"initializations\":%d}' ).\n%s", initIndex, initialization.String())
	} else {
		selectedInitialization.WriteString("CASE shard.\n")
		seen := map[string]bool{}
		for _, shard := range manifest.Shards {
			if shard.Name == "" || seen[shard.Name] || len(shard.Initialization) == 0 {
				return nil, fmt.Errorf("invalid test shard initialization: %q", shard.Name)
			}
			seen[shard.Name] = true
			initialization.Reset()
			initIndex = 0
			addOrder(shard.Initialization)
			fmt.Fprintf(&selectedInitialization, "WHEN %s.\ncheckpoint( '{\"kind\":\"plan\",\"initializations\":%d}' ).\n%s", abapTestLiteral(shard.Name), initIndex, initialization.String())
		}
		// An unknown selector produces no plan; the parent fails closed.
		selectedInitialization.WriteString("WHEN OTHERS.\nRETURN.\nENDCASE.\n")
	}
	if _, ok := files[assertionClass+".clas.abap"]; !ok {
		return nil, fmt.Errorf("test closure has no assertion exception marker")
	}
	replacements := strings.NewReplacer(
		"INITIALIZATION", selectedInitialization.String(),
		"HOSTDECLARATIONS", hostDeclarations.String(), "HOSTMETHODS", hostMethods.String(),
		"RUNTIME", n(runtime.Name), "TESTS", n("member.tests"), "FOCUSED", n("member.focused"),
		"TESTCASE", n("runtime/mocha.ts.TestCase"), "TITLE", n("member.title"), "SKIPPED", n("member.skipped"), "ONLY", n("member.only"),
		"BODY", n("member.body"), "CALL", n("member.call"),
		"PROMISECLASS", n("runtime."+hir.T(hir.Promise, hir.T(hir.Void)).String()), "SUITES", n("member.suites"), "SUITECLASS", n("runtime/mocha.ts.TestSuite"),
		"BEFOREALL", n("member.beforeAll"), "AFTERALL", n("member.afterAll"), "BEFOREEACH", n("member.beforeEach"), "AFTEREACH", n("member.afterEach"),
		"TRAPCLASS", n("exception.unexecuted"), "ASSERTCLASS", assertionClass, "ERRORMESSAGE", n("member.message"),
	)
	runner := replacements.Replace(abapTestRunner)
	if client != nil {
		runner = strings.Replace(runner, "METHOD run.\n", "METHOD run.\n"+n("runtime.classvalue.factory")+"=>extension = NEW zcl_lnt_test_extension( ).\n", 1)
	}
	formatted, err := abap.Format(runner)
	if err != nil {
		return nil, err
	}
	files["zcl_abapiti_test_runner.clas.abap"] = formatted
	sourceMap, err := abap.SourceMap(lowering.Prog, names)
	if err != nil {
		return nil, err
	}
	if client != nil {
		var entries map[string]abap.SourceEntry
		if err := json.Unmarshal(sourceMap, &entries); err != nil {
			return nil, err
		}
		for _, name := range []string{"ZCL_LNT_TEST_FACTORY", "ZCL_LNT_TEST_DELETE", "ZCL_LNT_TEST_EXTENSION", "ZIF_LNT_RUNTIME_EXTENSION"} {
			entries[name] = abap.SourceEntry{ID: "generated." + name, Kind: "runtime", Source: "hir/abap/test_packages.go"}
		}
		sourceMap, err = json.MarshalIndent(entries, "", " ")
		if err != nil {
			return nil, err
		}
	}
	files["names.json"] = string(sourceMap)
	return files, nil
}

// ModuleInitialization is one explicit initializer call: ABAP class
// constructors are lazy, so module state is initialized in manifest order.
type ModuleInitialization struct {
	File, Class string
	// Name is the hir.Names key of the static method to call on Class.
	Name string
}

// ModuleInitializations lists, per file in order, the module's static
// initializer and (with tests) its test-registration initializers.
func ModuleInitializations(modules map[string]*hir.Class, order []string, tests bool) []ModuleInitialization {
	var out []ModuleInitialization
	for _, file := range order {
		c := modules[file+".module"]
		if c == nil {
			continue
		}
		for _, m := range c.Methods {
			if m.Static && m.Name == "class_constructor" {
				out = append(out, ModuleInitialization{File: file, Class: c.Name, Name: "builtin.initialize." + c.Name})
			}
		}
		if !tests {
			continue
		}
		for _, m := range c.Methods {
			if isTestInitialization(m) {
				out = append(out, ModuleInitialization{File: file, Class: c.Name, Name: "member." + m.Name})
			}
		}
	}
	return out
}

func abapTestLiteral(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

const abapTestRunner = `CLASS zcl_abapiti_test_runner DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CLASS-METHODS run IMPORTING start_index TYPE i skip_init TYPE string shard TYPE string RETURNING VALUE(result) TYPE string.
CLASS-METHODS checkpoint IMPORTING value TYPE string.
PRIVATE SECTION.
HOSTDECLARATIONS
CLASS-METHODS host_body IMPORTING test TYPE REF TO TESTCASE EXPORTING settled TYPE REF TO PROMISECLASS message TYPE string.
CLASS-DATA initialization TYPE string.
CLASS-METHODS quote IMPORTING value TYPE string RETURNING VALUE(result) TYPE string.
CLASS-METHODS add_initialization IMPORTING file TYPE string action TYPE string status TYPE string message TYPE string.
ENDCLASS.
CLASS zcl_abapiti_test_runner IMPLEMENTATION.
METHOD checkpoint.
ENDMETHOD.
METHOD quote.
result = '"' && escape( val = value format = cl_abap_format=>e_json_string ) && '"'.
ENDMETHOD.
METHOD add_initialization.
IF initialization IS NOT INITIAL.
initialization = initialization && ','.
ENDIF.
DATA record TYPE string.
record = '{"file":' && quote( file ) && ',"action":' && quote( action ) && ',"message":' && quote( message ) && ',"status":' && quote( status ) && '}'.
initialization = initialization && record.
checkpoint( '{"kind":"initialization","record":' && record && '}' ).
ENDMETHOD.
HOSTMETHODS
METHOD host_body.
settled = test->BODY->CALL( ).
ENDMETHOD.
METHOD run.
DATA init_error TYPE REF TO cx_root.
DATA test_error TYPE REF TO cx_root.
DATA error_class TYPE string.
DATA error_text TYPE string.
DATA init_assertion TYPE REF TO ASSERTCLASS.
DATA init_trap TYPE REF TO TRAPCLASS.
DATA passed TYPE i.
DATA failed TYPE i.
DATA trapped TYPE i.
DATA not_run TYPE i.
DATA async_pending TYPE i.
DATA records TYPE string.
DATA record TYPE string.
DATA titles TYPE string.
DATA test_index TYPE i.
DATA status TYPE string.
DATA message TYPE string.
DATA passed_json TYPE string.
DATA test TYPE REF TO TESTCASE.
DATA suite TYPE REF TO SUITECLASS.
DATA settled TYPE REF TO PROMISECLASS.
CLEAR initialization.
INITIALIZATION
LOOP AT RUNTIME=>TESTS->items INTO DATA(registration).
test ?= registration.
IF titles IS NOT INITIAL.
titles = titles && ','.
ENDIF.
titles = titles && quote( test->TITLE ).
ENDLOOP.
checkpoint( '{"kind":"registry","titles":[' && titles && ']}' ).
LOOP AT RUNTIME=>TESTS->items INTO DATA(item).
test_index = test_index + 1.
IF test_index < start_index.
CONTINUE.
ENDIF.
test ?= item.
checkpoint( '{"kind":"test","index":' && |{ test_index }| && ',"title":' && quote( test->TITLE ) && '}' ).
status = 'passed'.
CLEAR message.
passed_json = 'false'.
IF test->SKIPPED = abap_true OR ( RUNTIME=>FOCUSED = abap_true AND test->ONLY = abap_false ).
status = 'not-run'.
ELSE.
TRY.
LOOP AT test->SUITES->items INTO DATA(suite_item).
suite ?= suite_item.
IF lines( suite->BEFOREALL->items ) + lines( suite->AFTERALL->items ) + lines( suite->BEFOREEACH->items ) + lines( suite->AFTEREACH->items ) > 0.
status = 'trapped'.
message = 'ABAP test runner: hook execution is not yet emitted'.
EXIT.
ENDIF.
ENDLOOP.
IF status = 'passed'.
host_body( EXPORTING test = test IMPORTING settled = settled message = message ).
IF message IS NOT INITIAL.
status = 'trapped'.
ELSE.
IF settled IS NOT BOUND.
DATA provider_trap TYPE REF TO TRAPCLASS.
provider_trap = NEW #( ).
provider_trap->source_location = 'await provider did not return a certified settled promise'.
RAISE EXCEPTION provider_trap.
ENDIF.
IF settled->rejection IS BOUND.
RAISE EXCEPTION settled->rejection.
ENDIF.
ENDIF.
ENDIF.
CATCH ASSERTCLASS INTO DATA(assertion).
status = 'failed'.
message = assertion->payload->ERRORMESSAGE.
CATCH TRAPCLASS INTO DATA(trap).
status = 'trapped'.
message = trap->source_location.
IF message CS 'unsupported-async' OR message CS 'certified settled promise'.
status = 'async-pending'.
ENDIF.
IF message CS 'unsupported-test-done'.
status = 'not-run'.
ENDIF.
CATCH cx_root INTO test_error.
status = 'trapped'.
error_class = cl_abap_classdescr=>get_class_name( test_error ).
REPLACE FIRST OCCURRENCE OF '\CLASS=' IN error_class WITH ''.
error_text = test_error->get_text( ).
message = error_class && |: | && error_text.
ENDTRY.
ENDIF.
CASE status.
WHEN 'passed'.
passed = passed + 1.
passed_json = 'true'.
WHEN 'failed'.
failed = failed + 1.
WHEN 'trapped'.
trapped = trapped + 1.
WHEN 'async-pending'.
async_pending = async_pending + 1.
WHEN OTHERS.
not_run = not_run + 1.
ENDCASE.
IF records IS NOT INITIAL.
records = records && ','.
ENDIF.
record = '{"title":' && quote( test->TITLE ) && ',"passed":' && passed_json && ',"message":' && quote( message ) && ',"status":' && quote( status ) && '}'.
records = records && record.
checkpoint( '{"kind":"result","index":' && |{ test_index }| && ',"record":' && record && '}' ).
ENDLOOP.
result = '{"asyncPending":' && |{ async_pending }| && ',"passed":' && |{ passed }| && ',"failed":' && |{ failed }| && ',"trapped":' && |{ trapped }| && ',"notRun":' && |{ not_run }| && ',"results":[' && records && '],"initialization":[' && initialization && ']}'.
ENDMETHOD.
ENDCLASS.
`
