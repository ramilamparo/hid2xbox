.PHONY: build test test-portable lint clean

GO := go
BIN := hid2xbox.exe

build:
	$(GO) build -o $(BIN) .

test:
	$(GO) test -v -count=1 ./...

test-portable:
	$(GO) test -v -count=1 -run "Test(Config|Normalize|Norm|Button|Contain|TUI)" ./...

lint:
	golangci-lint run ./...

docker-test:
	docker build --target test -t hid2xbox-test .

docker-build:
	docker build --target build -t hid2xbox-build .

clean:
	rm -f $(BIN)
