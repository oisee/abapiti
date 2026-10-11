package tsfront

import (
	"strings"
	"testing"
)

func TestDriverNamesMode(t *testing.T) {
	readable := DriverNames{"zcl_lnt_registry_osg", "zcl_lnt_registry_a4h", "zlnt_registry_run", "zlnt_registry_clean", "zlnt_registry_neg"}
	hashed := DriverNames{"zcl_abapiti_registry_run", "zcl_abapiti_registry_a4h", "zabapiti_registry_run", "zabapiti_registry_clean", "zabapiti_registry_neg"}
	for _, tc := range []struct {
		mode string
		want DriverNames
	}{{"", readable}, {"readable", readable}, {"hashed", hashed}} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("ABAPITI_NAMES", tc.mode)
			got, err := Drivers()
			if err != nil || got != tc.want {
				t.Fatalf("Drivers() = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
	t.Setenv("ABAPITI_NAMES", "typo")
	if _, err := Drivers(); err == nil || !strings.Contains(err.Error(), "ABAPITI_NAMES") {
		t.Fatalf("invalid naming mode: %v", err)
	}
}
