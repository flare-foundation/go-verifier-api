package test_util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/pkg/errors"
)

type TestCase[T any, R any] struct {
	TestName       string
	Input          T
	ExpectedValue  R
	ExpectError    bool
	ExpectedErrMsg string
}

func EncodeFTDCPMWMultisigAccountConfiguredRequest(data connector.IPMWMultisigAccountConfiguredRequestBody) ([]byte, error) {
	return structs.Encode(connector.AttestationTypeArguments[connector.PMWMultisigAccountConfigured].Request, data)
}

func DecodeFTDCTeeAvailabilityCheckResponse(data []byte) (connector.IPMWMultisigAccountConfiguredResponseBody, error) {
	var request connector.IPMWMultisigAccountConfiguredResponseBody
	err := structs.DecodeTo(connector.AttestationTypeArguments[connector.PMWMultisigAccountConfigured].Response, data, &request)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, errors.Errorf("%s", err)
	}

	return request, nil
}

func EncodeFTDCPMVPaymentStatusRequest(data connector.IPMWPaymentStatusRequestBody) ([]byte, error) {
	return structs.Encode(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Request, data)
}

func DecodeFTDCPMVPaymentStatusResponse(data []byte) (connector.IPMWPaymentStatusResponseBody, error) {
	var request connector.IPMWPaymentStatusResponseBody
	err := structs.DecodeTo(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Response, data, &request)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, errors.Errorf("%s", err)
	}

	return request, nil
}

func MakePostRequest(t *testing.T, url string, data interface{}, apiKey string) ([]byte, error) {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
