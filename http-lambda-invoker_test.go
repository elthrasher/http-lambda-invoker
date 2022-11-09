package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
)

type exchange struct {
	Request  events.APIGatewayProxyRequest
	Response events.APIGatewayProxyResponse
}

type mockLambdaClient struct {
	lambdaiface.LambdaAPI
	Resp lambda.InvokeOutput
}

func (m mockLambdaClient) Invoke(*lambda.InvokeInput) (*lambda.InvokeOutput, error) {
	return &m.Resp, nil
}

func runTest(t *testing.T, e exchange) {
	request, response := e.Request, e.Response
	req, err := http.NewRequest(request.HTTPMethod, request.Path, ioutil.NopCloser(strings.NewReader(request.Body)))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
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

	// Body equals mocked response
	if b := rr.Body.String(); b != response.Body {
		t.Errorf("handler returned unexpected body: got %v want %v",
			b, response.Body)
	}

	// Status code equals mocked response
	if s := rr.Code; s != response.StatusCode {
		t.Errorf("handler returned wrong status code: got %v want %v",
			s, response.StatusCode)
	}

	// Check CORS header
	if cors := rr.Header().Get(("Access-Control-Allow-Origin")); cors != "*" {
		t.Errorf("handler returned unexpected cors header: got %v want *", cors)
	}

	// Check content-type header
	if contentType := rr.Header().Get(("Content-Type")); contentType != response.Headers["content-type"] {
		t.Errorf("handler returned unexpected content-type header: got %v want %v", contentType, response.Headers["content-type"])
	}

	// No content-length header
	if l := rr.Header().Get(("content-length")); l != "" {
		t.Errorf("handler returned unexpected cors header: got %v want ''", l)
	}
}

func TestLambdaInvoke(t *testing.T) {

	responses := []exchange{
		{
			Request: events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Path:       "/",
			},
			Response: events.APIGatewayProxyResponse{
				Body:       `{"hasPayload":true}`,
				Headers:    nil,
				StatusCode: 200,
			},
		},
		{
			Request: events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Path:       "/props",
			},
			Response: events.APIGatewayProxyResponse{
				Body:       `{"hasPayload":true, "AnotherProp":123}`,
				Headers:    map[string]string{"content-type": "application/json"},
				StatusCode: 200,
			},
		},
		{
			Request: events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Path:       "/error",
			},
			Response: events.APIGatewayProxyResponse{
				Body:       `{"error":true}`,
				Headers:    nil,
				StatusCode: 500,
			},
		},
		{
			Request: events.APIGatewayProxyRequest{
				Body:       `{"prop":"value"}`,
				HTTPMethod: "POST",
				Path:       "/post",
			},
			Response: events.APIGatewayProxyResponse{
				Body:       "",
				Headers:    nil,
				StatusCode: 200,
			},
		},
	}

	for _, response := range responses {
		runTest(t, response)
	}
}

func Test_pathPatternToPathRegex(t *testing.T) {
	type args struct {
		pattern string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"empty path pattern",
			args{""},
			"",
			false,
		},
		{
			"path without variables",
			args{"/path/subpath"},
			"/path/subpath",
			false,
		},
		{
			"path with a variable",
			args{"/path/:pathid/subpath"},
			"/path/(?P<pathid>[^/]+)/subpath",
			false,
		},
		{
			"path with two variables",
			args{"/path/:pathid/subpath/:subpathid"},
			"/path/(?P<pathid>[^/]+)/subpath/(?P<subpathid>[^/]+)",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pathPatternToPathRegex(tt.args.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("pathPatternToPathRegex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.String() != tt.want {
				t.Errorf("pathPatternToPathRegex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractPathParameters(t *testing.T) {
	type args struct {
		path          string
		rePathPattern *regexp.Regexp
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			"path with no variable without regex",
			args{
				path:          "/path/subPath",
				rePathPattern: regexp.MustCompile(``),
			},
			map[string]string{},
		},
		{
			"path with no variable",
			args{
				path:          "/path/subPath",
				rePathPattern: regexp.MustCompile(`/path/subPath`),
			},
			map[string]string{},
		},
		{
			"path not matching the regexp",
			args{
				path:          "/foo",
				rePathPattern: regexp.MustCompile(`/path/(?P<pathid>[^/]+)`),
			},
			map[string]string{},
		},
		{
			"path with 2 variables",
			args{
				path:          "/path/12345/subPath/abcde",
				rePathPattern: regexp.MustCompile(`/path/(?P<pathid>[^/]+)/subPath/(?P<subpathid>[^/]+)`),
			},
			map[string]string{
				"pathid":    "12345",
				"subpathid": "abcde",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractPathParameters(tt.args.path, tt.args.rePathPattern); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractPathParameters() = %v, want %v", got, tt.want)
			}
		})
	}
}
