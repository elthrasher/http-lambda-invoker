package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"

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

	route := getConfig("ROUTE")
	rePathPattern, err := pathPatternToPathRegex(route)
	if err != nil {
		handleError(w, err)
		return
	}
	pathParameters := extractPathParameters(r.URL.Path, rePathPattern)

	// Get struct.
	request := events.APIGatewayProxyRequest{
		Body:                            string(body),
		HTTPMethod:                      r.Method,
		Path:                            r.URL.Path,
		MultiValueHeaders:               r.Header,
		Headers:                         multiValueMapToSingleValueMap(r.Header),
		MultiValueQueryStringParameters: r.URL.Query(),
		QueryStringParameters:           multiValueMapToSingleValueMap(r.URL.Query()),
		PathParameters:                  pathParameters,
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

// Convert a multi value map to a single value map. This is useful to convert multi value headers or query params to their single value counterparts.
// This function follows AWS rules: "With the default format, the load balancer uses the last value sent by the client"
// See: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/lambda-functions.html#multi-value-headers
func multiValueMapToSingleValueMap(m map[string][]string) map[string]string {
	ret := make(map[string]string, len(m))
	for k, v := range m {
		ret[k] = ""
		if len(v) > 0 {
			ret[k] = v[len(v)-1]
		}
	}
	return ret
}

// Convert a path pattern to a regexp. This is used to extract path parameters
// Example:
//
//	/path/:pathID/subPath/:subPathID
//
// is converted to:
//
//	/path/(?P<pathID>[^/]+)/subPath/(?P<subPathID>[^/]+)
func pathPatternToPathRegex(pattern string) (*regexp.Regexp, error) {
	rePathPatternToPathRegex := regexp.MustCompile(`:([^/]+)`)
	return regexp.Compile(rePathPatternToPathRegex.ReplaceAllString(pattern, `(?P<$1>[^/]+)`))
}

// Extract the path parameters from a real path according to a pattern
// Example:
//
// /path/12345/subPath/abcde
// matched with: /path/(?P<pathid>[^/]+)/subPath/(?P<subpathid>[^/]+)
// will return: {"pathid": "12345", "subpathid": "abcde"}
func extractPathParameters(path string, rePathPattern *regexp.Regexp) map[string]string {
	match := rePathPattern.FindStringSubmatch(path)
	pathParameters := map[string]string{}
	for i, name := range rePathPattern.SubexpNames() {
		if len(match) < i+1 {
			break
		}
		if i != 0 && name != "" {
			pathParameters[name] = match[i]
		}
	}
	return pathParameters
}
