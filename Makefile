TARGETS		:= facmod
GO_SOURCES	:= $(wildcard httputil/*.go) \
		   $(wildcard mods/*.go) \
		   $(wildcard xdg/*.go)
GO_MODULE	:= $(shell awk '/^module/ { print $$2 }' < go.mod)

all: $(TARGETS) README.html

facmod: $(wildcard cmd/facmod/*.go) $(GO_SOURCES)
	go build -o $@ $(GO_MODULE)/cmd/$@

README.html: README.adoc
	asciidoctor $<

.PHONY: clean
clean:
	-rm $(TARGETS)
