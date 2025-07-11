package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
	"gitlab.com/urskak/verifier-api/internal/api/validation"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", validation.IsHash32)
	validate.RegisterValidation("eth_addr", validation.IsCommonAddress)
}

func genericVerifyHandler[Req any, Res any](c *gin.Context, verifier verifierinterface.VerifierInterface[Req, Res]) {
	var input attestationtypes.AttestationRequest[Req]
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := validate.Struct(input); err != nil {
		c.JSON(400, gin.H{"error": "Validation failed: " + err.Error()})
		return
	}

	status, res, _ := verifier.Verify(c.Request.Context(), input.RequestBody)
	// TODO - what do to with the error?
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	return
	// }

	response := attestationtypes.AttestationResponse[Req, Res]{
		AttestationType: input.AttestationType,
		SourceID:        input.SourceID,
		RequestBody:     input.RequestBody,
		ResponseBody:    res,
	}

	c.JSON(http.StatusOK, attestationtypes.FullAttestationResponse[Req, Res]{
		AttestationStatus: status,
		Response:          response,
	})
}
