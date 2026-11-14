.PHONY: build run test vet clean

# binaries go to bin/ instead of next to the source
build:
	go build -o bin/rhapsody-rosarium .

run:
	go run .

test:
	go test ./...

vet:
	gofmt -l .
	go vet ./...

clean:
	rm -rf bin
