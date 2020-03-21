FROM golang:alpine AS build-env
RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh
WORKDIR /app 
RUN go get github.com/aws/aws-sdk-go
ADD . .
RUN go build -o main .

FROM alpine
WORKDIR /app
COPY --from=build-env /app/main /app/
RUN adduser -S -D -H -h /app appuser
USER appuser
EXPOSE 8088
CMD ["./main"]
