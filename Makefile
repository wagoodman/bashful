.DEFAULT_GOAL := ci
TARGETS := $(shell ls scripts) 

# ./bin/bashful example/00-demo.yml
# ./bin/bashful example/01-simple.yml
# ./bin/bashful example/02-simple-and-pretty.yml
# ./bin/bashful example/03-repetative.yml
# ./bin/bashful example/04-repetative-parallel.yml
# ./bin/bashful example/05-minimal.yml
# ./bin/bashful example/06-with-errors.yml
# ./bin/bashful example/07-vintage.yml
# ./bin/bashful example/08-complicated.yml
# ./bin/bashful example/09-stress-and-flow-control.yml
# ./bin/bashful example/10-bad-values.yml
# ./bin/bashful example/11-from-url.yml

$(TARGETS): 
	./scripts/$@

run: clean build
	./bin/bashful example/11-from-url.yml

clean:
	rm -f bin/bashful build.log

.PHONY: run clean $(TARGETS)