.PHONY: vendor build run clean

all: vendor build run clean

vendor:
	go get ./...

build: vendor
	go build -v

run: clean
	#clear
	#go run *.go example/segexample.yaml 
	go build && ./bashful example/08-complicated.yml

clean:
	rm -f bashful