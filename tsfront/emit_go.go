package tsfront

import (
	"fmt"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
)

// EmitGoWithTraps emits a verified program, replacing source-attributable Go
// backend refusals with located body traps. Unattributable failures still
// refuse emission. The lowered program is updated for evidence reporting.
func EmitGoWithTraps(lowering *RegistryLowering) (map[string]string, error) {
	if err := lowering.Err(); err != nil {
		return nil, err
	}
	var err error
	var files map[string]string
	for {
		files, err = gohir.Emit(lowering.Prog)
		if err == nil {
			break
		}
		e, ok := err.(hir.Error)
		if !ok || e.ID == 0 {
			return nil, err
		}
		trapped := false
		for _, c := range lowering.Prog.Classes {
			for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
				if m != nil && bodyIDs(m.Body)[e.ID] {
					m.Body = failureBody(m.Node, e.Error())
					trapped = true
				}
			}
		}
		if !trapped {
			return nil, err
		}
		(&lowerer{out: lowering.Prog}).repairFailedBodies()
		if errors := hir.Verify(lowering.Prog); len(errors) > 0 {
			return nil, fmt.Errorf("after Go failure trap: %s", errors[0])
		}
	}
	return files, nil
}
