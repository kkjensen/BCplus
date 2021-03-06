include VERSION

.PHONY: data

#PACK:=BCplus-$(major).$(minor).$(bugfix)$(quality).zip

#all: data godepgraph.svg
all: data

data:
	$(MAKE) -C data all

%.svg: %.dot
	dot -Tsvg $< > $@

.PHONY: godepgraph.dot
godepgraph.dot: Makefile
	godepgraph -l 2 -horizontal github.com/CmdrVasquess/BCplus > $@

pack: pack/pack BCplus
	pack/pack -pack zip

BCplus: core/version.go
	go build

core/version.go: VERSION
	go generate

pack/pack: $(wildcard pack/*.go)
	cd pack; go build

pack/vbcp.go: VERSION
	cd pack; go generate
