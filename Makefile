BINARY_NAME := outbound-link-checker

build_debug:
	GO111MODULE=on go build -v -o bin/${BINARY_NAME} *.go

build_prod:
	# Shrink binary by removing symbol and DWARF table
	# Ref: https://lukeeckley.com/post/useful-go-build-flags/
	GO111MODULE=on go build -v -ldflags="-s -w" -o bin/${BINARY_NAME} *.go

go_lint:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go vet .
	go tool fix .
	golangci-lint run

update_go_deps:
	GO111MODULE=on go get -t -u ./...

lint: format go_lint

format:
	go fmt .

clean:
	GO111MODULE=on go clean --modcache
	rm -rf bin/*

test:
	GO111MODULE=on go test . -v
