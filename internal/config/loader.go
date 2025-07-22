package config

import (
	"sync"

	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	teeavailabilitycheckconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"
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
