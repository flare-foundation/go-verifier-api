package verifier

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The hwmodel set is a security boundary, not configuration: it must contain
// exactly the confidential-memory technologies Google Confidential Space
// attests, and never GCP_SHIELDED_VM. Editing it should fail this
// test so the change is a conscious, reviewed act.
func TestConfidentialHWModelsAreExactlyTheGoogleCVMTechnologies(t *testing.T) {
	want := map[string]struct{}{
		"GCP_AMD_SEV":    {},
		"GCP_AMD_SEV_ES": {},
		"GCP_INTEL_TDX":  {},
	}
	require.Equal(t, want, confidentialHWModels)
	require.NotContains(t, confidentialHWModels, "GCP_SHIELDED_VM")
}
