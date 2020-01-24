SHELL = bash
default: build

.PHONY: build
build:
	@echo "Building autoscaler..."
	@go build
	@echo "Done"

.PHONY: plugins
plugins:
	@for d in apm/plugins; do \
	   for p in $${d}/*; do \
		   plugin=$$(basename $$p); \
			 echo "Building $${plugin}..."; \
			 go build -o ./plugins/$$plugin ./$$p/main.go; \
     done \
   done
	@echo "Done"

