.DEFAULT_GOAL := ci
TARGETS := $(shell ls scripts) 
.PHONY: run clean $(TARGETS)

$(TARGETS): 
	./scripts/$@

run: #clean build
	./bin/bashful run example/11-tags.yml --tags some-app1

examples:
	./bin/bashful run example/00-demo.yml
	./bin/bashful run example/01-simple.yml
	./bin/bashful run example/02-simple-and-pretty.yml
	./bin/bashful run example/03-repetitive.yml
	./bin/bashful run example/04-repetitive-parallel.yml
	./bin/bashful run example/05-minimal.yml
	./bin/bashful run example/06-with-errors.yml
	./bin/bashful run example/07-from-url.yml
	./bin/bashful run example/08-complicated.yml
	./bin/bashful run example/09-stress-and-flow-control.yml
	./bin/bashful run example/10-bad-values.yml || true

clean:
	rm -f bin/bashful build.log
