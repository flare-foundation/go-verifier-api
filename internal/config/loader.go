package config

import (
	"fmt"
	"os"
	"sync"

	pmwpaymentstatusconfig "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status/config"
	teeavailabilitycheckconfig "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/config"
	types "gitlab.com/urskak/verifier-api/internal/common"
)

var (
	pmyPaymentStatusConfig     *pmwpaymentstatusconfig.PMWPaymentStatusConfig
	pmyPaymentStatusConfigOnce sync.Once
	pmyPaymentStatusConfigErr  error

	teeAvailabilityCheckConfig     *teeavailabilitycheckconfig.TeeAvailabilityCheckConfig
	teeAvailabilityCheckConfigOnce sync.Once
	teeAvailabilityCheckConfigErr  error
)

func GetPMWPaymentStatusConfig() (*pmwpaymentstatusconfig.PMWPaymentStatusConfig, error) {
	pmyPaymentStatusConfigOnce.Do(func() {
		pmyPaymentStatusConfig, pmyPaymentStatusConfigErr = pmwpaymentstatusconfig.LoadPMWPaymentStatusConfig()
	})
	return pmyPaymentStatusConfig, pmyPaymentStatusConfigErr
}

func GetTeeAvailabilityCheckConfig() (*teeavailabilitycheckconfig.TeeAvailabilityCheckConfig, error) {
	teeAvailabilityCheckConfigOnce.Do(func() {
		teeAvailabilityCheckConfig, teeAvailabilityCheckConfigErr = teeavailabilitycheckconfig.LoadTeeAvailabilityCheckConfig()
	})
	return teeAvailabilityCheckConfig, teeAvailabilityCheckConfigErr
}

func LoadAttestationConfig() (any, error) {
	verifierType := os.Getenv("VERIFIER_TYPE")
	switch verifierType {
	case string(types.PMWPaymentStatus):
		return GetPMWPaymentStatusConfig()
	case string(types.TeeAvailabilityCheck):
		return GetTeeAvailabilityCheckConfig()
	default:
		return nil, fmt.Errorf("unsupported VERIFIER_TYPE: %s", verifierType)
	}
}
