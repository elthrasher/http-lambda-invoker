package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
)

type mockLambdaClient struct {
	lambdaiface.LambdaAPI
	Resp lambda.InvokeOutput
}

func (m mockLambdaClient) Invoke(*lambda.InvokeInput) (*lambda.InvokeOutput, error) {
	return &m.Resp, nil
}

func TestLambdaInvoke(t *testing.T) {
	req, err := http.NewRequest("GET", "/", ioutil.NopCloser(strings.NewReader("")))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	response := restResponse{
		Body:       "this is a test",
		Headers:    nil,
		StatusCode: 200,
	}
	payload, err := json.Marshal(response)
	status := int64(200)
	resp := struct {
		Resp lambda.InvokeOutput
	}{
		Resp: lambda.InvokeOutput{
			Payload:    payload,
			StatusCode: &status,
		},
	}

	l := LambdaClient{
		mockLambdaClient{Resp: resp.Resp},
	}

	l.invokeLambda(rr, req)

	if s := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			s, http.StatusOK)
	}

	expected := `this is a test`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
