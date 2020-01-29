SHELL = bash
default: build

.PHONY: build
build:
	@echo "Building autoscaler..."
	@go build
	@echo "Done"

.PHONY: plugins
plugins:
	@for d in apm/plugins target/plugins strategy/plugins; do \
	   for p in $${d}/*; do \
		   plugin=$$(basename $$p); \
			 echo "Building $${plugin}..."; \
			 pushd $$p > /dev/null; \
			 go build -o ../../../plugins/$$plugin; \
			 popd > /dev/null;  \
     done; \
   done
	@echo "Done"

