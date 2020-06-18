SHELL = bash

define build
	@echo "==> Building $@"
	@cd ./$$(basename $@) && go build -o ../$@
	@echo "==> Done"
endef

default: clean bin/noop-strategy bin/noop-fake-strategy

bin/noop-strategy: bin
	$(build)

bin/noop-fake-strategy: bin
	@echo "==> Building $@"
	@cd ./noop-strategy && go build -o ../$@
	@echo "==> Done"

bin:
	@mkdir -p ./bin

clean:
	@rm -f ./bin/*

