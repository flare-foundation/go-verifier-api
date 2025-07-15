package api

import (
	"fmt"
	"net/http"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
	attestationutils "gitlab.com/urskak/verifier-api/internal/api/utils"
	"gitlab.com/urskak/verifier-api/internal/api/validation"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", validation.IsHash32)
	validate.RegisterValidation("eth_addr", validation.IsCommonAddress)
}

func genericVerifyHandler[Req any, Res any](c *gin.Context, verifier verifierinterface.VerifierInterface[Req, Res], attestationType connector.AttestationType, sourceID string) {
	var request attestationtypes.AttestationRequest[Req]
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validate.Struct(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed: " + err.Error()})
		return
	}
	verifierAttestationNameEnc, err := attestationutils.EncodeAttestationOrSourceName(string(attestationType))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Attestation type name encoding failed: " + err.Error()})
		return
	}
	verifierSourceNameEnc, err := attestationutils.EncodeAttestationOrSourceName(string(sourceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Source name encoding failed: " + err.Error()})
		return
	}
	if request.AttestationType != verifierAttestationNameEnc || request.SourceID != verifierSourceNameEnc {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).", request.AttestationType, request.SourceID, string(attestationType), verifierAttestationNameEnc, string(sourceID), verifierSourceNameEnc)})
		return
	}

	status, res, _ := verifier.Verify(c.Request.Context(), request.RequestBody)
	// TODO - what do to with the error?
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	return
	// }

	response := attestationtypes.AttestationResponse[Req, Res]{
		AttestationType: request.AttestationType,
		SourceID:        request.SourceID,
		RequestBody:     request.RequestBody,
		ResponseBody:    res,
	}

	c.JSON(http.StatusOK, attestationtypes.FullAttestationResponse[Req, Res]{
		AttestationStatus: status,
		Response:          response,
	})
}
