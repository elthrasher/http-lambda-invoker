# http-lambda-invoker

A tiny (<20m) Docker image to invoke lambda functions via http. This is my first attempt at golang, so appreciate any course correction!

[SAM](https://aws.amazon.com/serverless/sam/) is a great framework for developing a few lambda functions, but it's challenging to fit into existing development workflows based on docker-compose. http-lambda-invoker allows invoking lambda functions via http in a mixed serverless and container local development environment.

Check out the [blog post](https://dev.to/elthrasher/integrating-sam-into-container-workflows-with-http-lambda-invoker-4o8) I wrote explaining why.

# Example of use

```yaml
version: '3.7'

services:
  api:
    container_name: api
    image: elthrasher/http-lambda-invoker
    environment:
      - LAMBDA_ENDPOINT=http://lambda:9001
      - LAMBDA_NAME=MyFunctionName
      - PORT=8080
    ports:
      - '8080:8080'
  lambda:
    container_name: lambda
    image: lambci/lambda:nodejs12.x
    command: app.MyFunctionName
    environment:
      - DOCKER_LAMBDA_STAY_OPEN=1
    volumes:
      - ./build:/var/task:ro,delegated
```

See also the [example repo](https://github.com/elthrasher/http-lambda-invoker-example).

# Environment Variables

* LAMBDA_ENDPOINT - This is the address and port of your [lambci](https://github.com/lambci/docker-lambda) docker container running your lambda function. It should probably reference an address in your docker network. In the provided example, it uses the service name plus default port for lambci. (required)
* LAMBDA_NAME - The name of the function you want to call. AWS is somewhat forgiving here. If you have only one function, the name doesn't matter, but it's still required. (required)
* PORT - The port you want to run http-lambda-invoker on. This should match the right-side ports mapping in the compose file if you want to hit it with a browser.

# http proxy

The path, query params, request body and headers will all be passed to your lambda function and then mapped into the response object.

# CORS

[CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) errors aren't fun in development environments so the proxy automatically sets `*` for `Access-Control-Allow-Origin`. This could be made configurable, but I'm not sure there's any need.

# Limitations

Only one function is supported per instance of http-lambda-invoker. If you wish to orchestrate multiple functions, you'll need one instance of http-lambda-invoker per function and you'd have to map each to a different port.

I'm interested in handling this limitation by having http-lambda-invoker parse the SAM/Cloudformation template similar to how the flask application included in SAM CLI works.

# API Gateway vs. HTTP Gateway

I'm still figuring this out myself, but would like to support both!

# Build it yourself!

`docker build . -t <some_tag>`

# Test it!

`go test`
