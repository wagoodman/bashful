.PHONY: test upload clean bootstrap

test:
	(. venv/bin/activate; \
	tox; \
	)

upload: #test
	(. venv/bin/activate; \
	python setup.py sdist upload; \
	make clean; \
	)

clean:
	rm -f MANIFEST
	rm -rf build dist

bootstrap: venv
	. venv/bin/activate
	venv/bin/pip install -e .
	venv/bin/pip install --upgrade tox
	make clean

venv:
	virtualenv venv
	venv/bin/pip install --upgrade pip
	venv/bin/pip install --upgrade setuptools
