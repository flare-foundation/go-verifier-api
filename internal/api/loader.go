package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	paymentservice "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status"
	teeavailabilitycheck "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/verifier"
	types "gitlab.com/urskak/verifier-api/internal/common"
	"gitlab.com/urskak/verifier-api/internal/config"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

func LoadModule(attType string, api huma.API, registry huma.Registry) error {
	switch attType {
	case string(connector.PMWPaymentStatus):
		service, err := paymentservice.NewPaymentService()
		if err != nil {
			return fmt.Errorf("failed to initialize payment service: %w", err)
		}
		v := service.GetVerifier()

		registerVerifier(api, registry, attType, v)
	case string(connector.AvailabilityCheck):
		cfg, err := config.GetTeeAvailabilityCheckConfig()
		if err != nil {
			return fmt.Errorf("failed to initialize tee config: %w", err)
		}
		v, err := teeavailabilitycheck.GetVerifier(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize payment service: %w", err)
		}
		registerVerifier(api, registry, attType, v)
	default:
		return fmt.Errorf("unsupported attestation type: %s", attType)
	}
	return nil
}

func registerVerifier[Req any, Res any](api huma.API, registry huma.Registry, attType string, verifier verifierinterface.VerifierInterface[Req, Res]) {
	reqSchema := registry.Schema(reflect.TypeOf(types.AttestationRequest[Req]{}), true, "")
	respSchema := registry.Schema(reflect.TypeOf(types.AttestationResponse[Req, Res]{}), true, "")

	huma.Register(api, huma.Operation{
		OperationID: fmt.Sprintf("postVerify_%s", attType),
		Summary:     fmt.Sprintf("Attestation for %s", attType),
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/verify", attType),
		RequestBody: &huma.RequestBody{
			Description: "Full attestation request",
			Required:    true,
			Content: map[string]*huma.MediaType{
				"application/json": {
					Schema: reqSchema,
				},
			},
		},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Attestation successful",
				Content: map[string]*huma.MediaType{
					"application/json": {
						Schema: respSchema,
					},
				},
			},
		},
	}, func(ctx context.Context, input *types.AttestationRequest[Req]) (*types.AttestationResponse[Req, Res], error) {
		res, err := verifier.Verify(ctx, input.RequestBody)
		if err != nil {
			return nil, err
		}
		return &types.AttestationResponse[Req, Res]{
			AttestationType: input.AttestationType,
			SourceID:        input.SourceID,
			RequestBody:     input.RequestBody,
			ResponseBody:    res,
		}, nil
	})
}
