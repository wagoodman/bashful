.DEFAULT_GOAL := ci
TARGETS := $(shell ls scripts) 

$(TARGETS): 
	./scripts/$@

run: clean build
	./bin/bashful example/01-simple.yml
	./bin/bashful example/02-simple-and-pretty.yml
	./bin/bashful example/03-repetative.yml
	./bin/bashful example/04-repetative-parallel.yml
	./bin/bashful example/05-minimal.yml
	./bin/bashful example/06-with-errors.yml
	./bin/bashful example/07-vintage.yml
	./bin/bashful example/08-complicated.yml
	./bin/bashful bad.yml
	
clean:
	rm -f bin/bashful build.log

.PHONY: run clean $(TARGETS)