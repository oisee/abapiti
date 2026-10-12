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
	files, names, err := abap.EmitTestNamed(lowering.Prog, assertionOwners)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Initialization []string `json:"initialization"`
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
	var initialization strings.Builder
	for _, file := range manifest.Initialization {
		c := modules[file+".module"]
		if c == nil {
			continue
		}
		actions := []string{}
		for _, m := range c.Methods {
			if m.Static && m.Name == "class_constructor" {
				actions = append(actions, n("builtin.initialize."+c.Name))
			}
		}
		for _, m := range c.Methods {
			if isTestInitialization(m) {
				actions = append(actions, n("member."+m.Name))
			}
		}
		for _, action := range actions {
			fmt.Fprintf(&initialization, "TRY.\n%s=>%s( ).\nCATCH %s INTO init_assertion.\nadd_initialization( file = %s action = %s status = 'failed' message = init_assertion->payload->%s ).\nCATCH %s INTO init_trap.\nadd_initialization( file = %s action = %s status = 'trapped' message = init_trap->source_location ).\nCATCH cx_root INTO init_error.\nadd_initialization( file = %s action = %s status = 'trapped' message = init_error->get_text( ) ).\nENDTRY.\n", n(c.Name), action, assertionClass, abapTestLiteral(file), abapTestLiteral(action), n("member.message"), n("exception.unexecuted"), abapTestLiteral(file), abapTestLiteral(action), abapTestLiteral(file), abapTestLiteral(action))
		}
	}
	// Force the runtime initializer even when a closure did not otherwise use it.
	runtime := modules["runtime/mocha.ts.TestRuntime"]
	for _, m := range runtime.Methods {
		if m.Static && m.Name == "class_constructor" {
			fmt.Fprintf(&initialization, "%s=>%s( ).\n", n(runtime.Name), n("builtin.initialize."+runtime.Name))
		}
	}
	if _, ok := files[assertionClass+".clas.abap"]; !ok {
		return nil, fmt.Errorf("test closure has no assertion exception marker")
	}
	replacements := strings.NewReplacer(
		"INITIALIZATION", initialization.String(),
		"RUNTIME", n(runtime.Name), "TESTS", n("member.tests"), "FOCUSED", n("member.focused"),
		"TESTCASE", n("runtime/mocha.ts.TestCase"), "TITLE", n("member.title"), "SKIPPED", n("member.skipped"), "ONLY", n("member.only"),
		"BODY", n("member.body"), "CALL", n("member.call"),
		"PROMISECLASS", n("runtime."+hir.T(hir.Promise, hir.T(hir.Void)).String()), "SUITES", n("member.suites"), "SUITECLASS", n("runtime/mocha.ts.TestSuite"),
		"BEFOREALL", n("member.beforeAll"), "AFTERALL", n("member.afterAll"), "BEFOREEACH", n("member.beforeEach"), "AFTEREACH", n("member.afterEach"),
		"TRAPCLASS", n("exception.unexecuted"), "ASSERTCLASS", assertionClass, "ERRORMESSAGE", n("member.message"),
	)
	runner := replacements.Replace(abapTestRunner)
	formatted, err := abap.Format(runner)
	if err != nil {
		return nil, err
	}
	files["zcl_abapiti_test_runner.clas.abap"] = formatted
	return files, nil
}

func abapTestLiteral(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

const abapTestRunner = `CLASS zcl_abapiti_test_runner DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CLASS-METHODS run RETURNING VALUE(result) TYPE string.
PRIVATE SECTION.
CLASS-DATA initialization TYPE string.
CLASS-METHODS quote IMPORTING value TYPE string RETURNING VALUE(result) TYPE string.
CLASS-METHODS add_initialization IMPORTING file TYPE string action TYPE string status TYPE string message TYPE string.
ENDCLASS.
CLASS zcl_abapiti_test_runner IMPLEMENTATION.
METHOD quote.
result = '"' && escape( val = value format = cl_abap_format=>e_json_string ) && '"'.
ENDMETHOD.
METHOD add_initialization.
IF initialization IS NOT INITIAL.
initialization = initialization && ','.
ENDIF.
initialization = initialization && '{"file":' && quote( file ) && ',"action":' && quote( action ) && ',"message":' && quote( message ) && ',"status":' && quote( status ) && '}'.
ENDMETHOD.
METHOD run.
DATA init_error TYPE REF TO cx_root.
DATA init_assertion TYPE REF TO ASSERTCLASS.
DATA init_trap TYPE REF TO TRAPCLASS.
DATA passed TYPE i.
DATA failed TYPE i.
DATA trapped TYPE i.
DATA not_run TYPE i.
DATA async_pending TYPE i.
DATA records TYPE string.
DATA status TYPE string.
DATA message TYPE string.
DATA passed_json TYPE string.
DATA test TYPE REF TO TESTCASE.
DATA suite TYPE REF TO SUITECLASS.
DATA settled TYPE REF TO PROMISECLASS.
CLEAR initialization.
INITIALIZATION
LOOP AT RUNTIME=>TESTS->items INTO DATA(item).
test ?= item.
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
settled = test->BODY->CALL( ).
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
CATCH cx_root INTO DATA(test_error).
status = 'trapped'.
message = test_error->get_text( ).
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
records = records && '{"title":' && quote( test->TITLE ) && ',"passed":' && passed_json && ',"message":' && quote( message ) && ',"status":' && quote( status ) && '}'.
ENDLOOP.
result = '{"asyncPending":' && |{ async_pending }| && ',"passed":' && |{ passed }| && ',"failed":' && |{ failed }| && ',"trapped":' && |{ trapped }| && ',"notRun":' && |{ not_run }| && ',"results":[' && records && '],"initialization":[' && initialization && ']}'.
ENDMETHOD.
ENDCLASS.
`
