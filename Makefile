.PHONY: vendor build run

all: vendor build run

vendor:
	go get ./...

build: vendor
	go build -v

run:
	clear
	go run main.go example/complicated.yml
	#go run main.go example/segexample.yaml
