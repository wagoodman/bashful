.DEFAULT_GOAL := ci
TARGETS := $(shell ls scripts) 

# ./bin/bashful run example/00-demo.yml
# ./bin/bashful run example/01-simple.yml
# ./bin/bashful run example/02-simple-and-pretty.yml
# ./bin/bashful run example/03-repetative.yml
# ./bin/bashful run example/04-repetative-parallel.yml
# ./bin/bashful run example/05-minimal.yml
# ./bin/bashful run example/06-with-errors.yml
# ./bin/bashful run example/07-vintage.yml
# ./bin/bashful run example/08-complicated.yml
# ./bin/bashful run example/09-stress-and-flow-control.yml
# ./bin/bashful run example/10-bad-values.yml
# ./bin/bashful run example/11-from-url.yml

$(TARGETS): 
	./scripts/$@

run: #clean build
	./bin/bashful bundle example/11-from-url.yml

clean:
	rm -f bin/bashful build.log

.PHONY: run clean $(TARGETS)