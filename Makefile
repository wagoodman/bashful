.PHONY: vendor build run

all: vendor build run

vendor:
	go get ./...

build: vendor
	go build -v

run:
	#clear
	#go run *.go example/segexample.yaml 
	go run *.go example/complicated.yml
