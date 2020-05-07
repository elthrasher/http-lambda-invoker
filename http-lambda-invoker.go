package main

import (
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// LambdaClient enables mocking of the client for test purposes
type LambdaClient struct {
	lambdaiface.LambdaAPI
}

type proxyHeader map[string]string

// Parts of the request to send to Lambda.
type makeProxyRequest struct {
	Body              []byte              `json:"body"`
	Headers           proxyHeader					`json:"headers"`
	HTTPMethod        string              `json:"httpMethod"`
	Path              string              `json:"path"`
	QueryStringParams map[string][]string `json:"queryStringParameters"`
}

// Parts of the response to send back to the caller.
type restResponse struct {
	Body       string
	Headers    map[string]string
	StatusCode int
}

// Set some defaults for envvars.
// Access key and secret should normally be ignored as we're calling a local function.
func getConfig(key string) string {
	c := os.Getenv(key)
	if c != "" {
		return c
	}
	switch key {
	case "AWS_ACCESS_KEY_ID":
		return "foo"
	case "AWS_SECRET_ACCESS_KEY":
		return "bar"
	case "AWS_REGION":
		return endpoints.UsEast1RegionID
	case "PORT":
		return "8080"
	default:
		return ""
	}
}

func makeProxyHeaders(originalHeaders map[string][]string) proxyHeader {
	var newHeaders = make(proxyHeader)

	for header := range originalHeaders {
		newHeaders[header] = strings.Join(originalHeaders[header], "")
	}

	return newHeaders
}

func handleError(w http.ResponseWriter, err error) {
	http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusBadRequest)
}

func handler(w http.ResponseWriter, r *http.Request) {

	// Create AWS session.
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(getConfig("AWS_ACCESS_KEY_ID"), getConfig("AWS_SECRET_ACCESS_KEY"), getConfig("AWS_SESSION_TOKEN")),
		Region:      aws.String(getConfig("AWS_REGION")),
		Endpoint:    aws.String(getConfig("LAMBDA_ENDPOINT")),
	}))

	// Initialize lambda client.
	c := LambdaClient{
		lambda.New(sess, &aws.Config{}),
	}

	c.invokeLambda(w, r)

}

func (c *LambdaClient) invokeLambda(w http.ResponseWriter, r *http.Request) {
	// Error handling seems really verbose. Is there a better way?

	// Read request body.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleError(w, err)
		return
	}

	// Convert headers to appropriate ApiGateway format
	proxyHeaders := makeProxyHeaders(r.Header)

	// Get struct.
	request := makeProxyRequest{body, proxyHeaders, r.Method, r.URL.Path, r.URL.Query()}

	// Marshal request.
	payload, err := json.Marshal(request)
	if err != nil {
		handleError(w, err)
		return
	}

	// Invoke Lambda.
	result, err := c.Invoke(&lambda.InvokeInput{FunctionName: aws.String(getConfig("LAMBDA_NAME")), Payload: payload})
	if err != nil {
		handleError(w, err)
		return
	}

	var response restResponse

	// Unmarshal response into `response`.
	err = json.Unmarshal(result.Payload, &response)
	if err != nil {
		handleError(w, err)
		return
	}

	// Add headers to ResponseWriter omitting content-length, which came back with the wrong length.
	for key, value := range response.Headers {
		if key != "content-length" {
			w.Header().Add(key, value)
		}
	}
	// Enable cors
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// Write status code and body.
	w.WriteHeader(response.StatusCode)
	fmt.Fprintf(w, string(response.Body))
}

// Start simple web server with configured port, sending all traffic to handler.
func main() {
	var Port = getConfig("PORT")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", Port), nil))
}
