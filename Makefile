export GO15VENDOREXPERIMENT=1

help:
	@echo "HELP:"
	@echo "make vendor"
	@echo "make assets"
	@echo "make build"
	@echo "make config"
	@echo "make run"

vendor:
	godep save ./...

assets:
	go get github.com/jteeuwen/go-bindata/go-bindata
	cd server && go-bindata -pkg server data

build: assets
	go build -o sogiboard .

run: build
	./runenv ./sogiboard

config: runenv

runenv:
	echo "#!/bin/sh" >$@
	node -e "var env = $$(heroku config --json); for(v in env) console.log('export '+v+'=\''+env[v].replace(/'/g, '\'\"\'\"\'')+'\'')" >>$@
	echo 'exec "$$@"' >>$@
	chmod +x $@

.PHONY: help assets vendor build run config runenv
