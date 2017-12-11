.DEFAULT_GOAL := ci
TARGETS := $(shell ls scripts) 

$(TARGETS): 
	./scripts/$@

run: clean build
	./bin/bashful example/08-complicated.yml
	
clean:
	rm -f bin/bashful build.log

.PHONY: run clean $(TARGETS)