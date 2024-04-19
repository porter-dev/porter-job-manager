# syntax=docker/dockerfile:1.1.7-experimental

# Environment to build manager binary
FROM golang:1.15.6-alpine3.12 as build
WORKDIR /porter

RUN apk update && apk add gcc musl-dev

COPY go.mod go.sum ./
COPY /cmd ./cmd

RUN go mod download

RUN go build -ldflags '-w -s' -a -o ./bin/manager ./cmd/manager

# Deployment environment
# ----------------------
FROM debian:bullseye-slim as runner
WORKDIR /porter

RUN apt-get update && apt-get install -y git && apt-get clean

COPY --from=build /porter/bin/manager /usr/bin/

ENTRYPOINT [ "manager" ]
