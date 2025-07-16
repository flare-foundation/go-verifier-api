package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/joho/godotenv"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
	attestationutils "gitlab.com/urskak/verifier-api/internal/api/utils"
)

func RunServer() {
	_ = godotenv.Load()

	verifierTypeStr := os.Getenv("VERIFIER_TYPE")
	port := os.Getenv("PORT")
	if verifierTypeStr == "" || port == "" {
		log.Fatal("VERIFIER_TYPE and PORT must be set")
	}
	verifierType, err := attestationutils.ParseAttestationType(verifierTypeStr)
	if err != nil {
		log.Fatalf("Invalid VERIFIER_TYPE in .env: %v", err)
	}

	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Verifier API", "1.0"))
	// Register GET /greeting/{name}
	huma.Register(api, huma.Operation{
		OperationID: "get-greeting",
		Method:      http.MethodGet,
		Path:        "/greeting/{name}",
		Summary:     "Get a greeting",
		Description: "Get a greeting for a person by name.",
		Tags:        []string{"Greetings"},
	}, func(ctx context.Context, input *struct {
		Name string `path:"name" maxLength:"30" example:"world" doc:"Name to greet"`
	}) (*GreetingOutput, error) {
		resp := &GreetingOutput{}
		resp.Body.Message = fmt.Sprintf("Hello, %s!", input.Name)
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "post-review",
		Method:        http.MethodPost,
		Path:          "/reviews",
		Summary:       "Post a review",
		Tags:          []string{"Reviews"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, i *ReviewInput) (*struct{}, error) {
		fmt.Println(i)
		// TODO: save review in data store.
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   fmt.Sprintf("postVerify_%s", verifierType),
		Summary:       fmt.Sprintf("Attestation for %s", verifierType),
		Method:        http.MethodPost,
		Path:          fmt.Sprintf("/%s/verify", verifierType),
		Tags:          []string{string(verifierType)},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, request *attestationtypes.AttestationRequestTeeAvailabilityCheck) (*attestationtypes.FullAttestationResponseTeeAvailabilityCheck, error) {
		fmt.Println(request)
		// verifierAttestationNameEnc, err := attestationutils.EncodeAttestationOrSourceName(string(verifierType))
		// if err != nil {
		// 	return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("attestation type name encoding failed: %v", err))
		// }
		// var sourceID = "tee" //TODO
		// verifierSourceNameEnc, err := attestationutils.EncodeAttestationOrSourceName(string(sourceID))
		// if err != nil {
		// 	return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("source name encoding failed: %v", err))
		// }
		// if request.Body.AttestationType != verifierAttestationNameEnc || request.SourceID != verifierSourceNameEnc {
		// 	return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf(
		// 		"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).",
		// 		request.AttestationType, request.SourceID,
		// 		string(verifierType), verifierAttestationNameEnc,
		// 		sourceID, verifierSourceNameEnc,
		// 	))
		// }

		// status, res, err := verifier.Verify(ctx, request.RequestBody)
		// response := attestationtypes.AttestationResponse[Req, Res]{
		// 	AttestationType: request.AttestationType,
		// 	SourceID:        request.SourceID,
		// 	RequestBody:     request.RequestBody,
		// 	ResponseBody:    res,
		// }

		return &attestationtypes.FullAttestationResponseTeeAvailabilityCheck{
			// AttestationStatus: status,
			// Response:          response,
		}, err // TODO separate error and none error - check underlying code
	})

	// err = LoadModule(router, verifierType)
	// if err != nil {
	// 	logger.Fatalf("Failed to load verifier module: %v", err)
	// }

	log.Printf("Starting server on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

type GreetingOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}
type ReviewInput struct {
	Body struct {
		Author  string `json:"author" maxLength:"10" doc:"Author of the review"`
		Rating  int    `json:"rating" minimum:"1" maximum:"5" doc:"Rating from 1 to 5"`
		Message string `json:"message,omitempty" maxLength:"100" doc:"Review message"`
	}
}
