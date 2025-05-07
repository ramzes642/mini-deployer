FROM golang:1.23-alpine AS build


RUN mkdir /go/src/deployer
WORKDIR /go/src/deployer

COPY go.mod ./
COPY main.go ./
COPY handler.go ./

RUN go build -o ./deployer .

FROM alpine:latest

RUN apk add --no-cache bash ca-certificates

COPY --from=build /go/src/deployer/deployer /deployer
COPY config.sample.json /etc/mini-deployer.json

EXPOSE 7654

ENTRYPOINT ["/deployer"]
