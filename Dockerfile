# Build stage
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /dev/null ./...

# Test stage — runs portable tests on Linux
FROM golang:1.23-alpine AS test
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test -v -count=1 -run "Test(Config|Normalize|Norm|Button|Contain|TUI)" ./...
