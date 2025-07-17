package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/go-playground/validator/v10"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
	attestationutils "gitlab.com/urskak/verifier-api/internal/api/utils"
	"gitlab.com/urskak/verifier-api/internal/api/validation"
	teecrypto "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/crypto"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", validation.IsHash32)
	validate.RegisterValidation("eth_addr", validation.IsCommonAddress)
}

func PMWPaymentStatusHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[attestationtypes.IPMWPaymentStatusRequestBody, attestationtypes.IPMWPaymentStatusResponseBody], sourceID string) {
	huma.Register(api, getHumaOperation(attestationType, "verify"), func(ctx context.Context, request *attestationtypes.IFtdcHubFtdcRequestHeaderPMWPaymentStatus) (*attestationtypes.IPMWPaymentStatusResponseBody, error) {
		fmt.Println(request)
		if err := validate.Struct(request); err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
		}
		err := attestationutils.ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId)
		if err != nil {
			return nil, err
		}

		res, err := verifier.Verify(ctx, attestationtypes.IPMWPaymentStatusRequestBody{}) //TODO decode requestBody
		return &res, err                                                                  // TODO separate error and none error - check underlying code  //TODO decode responseBody
	})
}

func TeeAvailabilityCheckHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[attestationtypes.ITeeAvailabilityCheckRequestBody, attestationtypes.ITeeAvailabilityCheckResponseBody], sourceID string) {
	huma.Register(api, getHumaOperation(attestationType, "verify"), func(ctx context.Context, request *attestationtypes.IFtdcHubFtdcRequestHeaderTeeAvailabilityCheck) (*struct{}, error) { //TODO struct? status field must be an int
		fmt.Println(request)
		if err := validate.Struct(request); err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
		}
		err := attestationutils.ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId)
		if err != nil {
			return nil, err
		}

		fmt.Println("HELLO1")
		_, err = verifier.Verify(ctx, attestationtypes.ITeeAvailabilityCheckRequestBody{}) //TODO decode requestBody
		fmt.Println("HELLO2")
		// return huma.Response(http.StatusOK, &res)
		return nil, err // TODO separate error and none error - check underlying code //TODO decode responseBody
	})

	huma.Register(api, getHumaOperation(attestationType, "prepareRequestBody"), func(ctx context.Context, request *attestationtypes.IFtdcHubFtdcRequestHeaderTeeAvailabilityCheck) (*attestationtypes.EncodedRequestBody, error) {
		fmt.Println(request)
		if err := validate.Struct(request); err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
		}
		err := attestationutils.ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId)
		if err != nil {
			return nil, err
		}
		// TODO also verify
		res, err := teecrypto.AbiEncodeRequestBody(request.Body.RequestBody)
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("encoding failed: %v", err))
		}
		fmt.Println("HELLO1")
		fmt.Println("HELLO2", hex.EncodeToString(res))
		return &attestationtypes.EncodedRequestBody{EncodedRequestBody: hex.EncodeToString(res)}, nil
	})
}

func getHumaOperation(attestationType connector.AttestationType, path string, model any) huma.Operation {
	return huma.Operation{
		OperationID:   fmt.Sprintf("post_%s_%s", attestationType, path),
		Summary:       fmt.Sprintf("Attestation for %s", attestationType),
		Method:        http.MethodPost,
		Path:          fmt.Sprintf("/%s/%s", attestationType, path),
		Tags:          []string{string(attestationType)},
		DefaultStatus: http.StatusOK,
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Successful response",
				Content: map[string]*huma.MediaType{
					"application/json": {
						Schema: openapi.SchemaFromExample(model),
					},
				},
			},
		},
	}
}
