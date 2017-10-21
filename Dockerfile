FROM golang:1.9.0-alpine

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh make