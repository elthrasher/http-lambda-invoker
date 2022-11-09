package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
)

// LambdaClient enables mocking of the client for test purposes
type LambdaClient struct {
	lambdaiface.LambdaAPI
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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, err)
		return
	}

	// Get struct.
	request := events.APIGatewayProxyRequest{
		Body:                            string(body),
		HTTPMethod:                      r.Method,
		Path:                            r.URL.Path,
		MultiValueHeaders:               r.Header,
		MultiValueQueryStringParameters: r.URL.Query(),
	}

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

	var response events.APIGatewayProxyResponse

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
	fmt.Fprint(w, string(response.Body))
}

// Start simple web server with configured port, sending all traffic to handler.
func main() {
	var Port = getConfig("PORT")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", Port), nil))
}
