FROM golang:1.20 as build


RUN mkdir /go/src/deployer
WORKDIR /go/src/deployer

COPY go.mod ./
COPY main.go ./

RUN go build -o ./deployer .

FROM alpine:latest

RUN apk add --no-cache bash ca-certificates

COPY --from=build /go/src/deployer/deployer /deployer
COPY config.sample.json /etc/config.json

EXPOSE 7654

ENTRYPOINT ["/deployer"]
