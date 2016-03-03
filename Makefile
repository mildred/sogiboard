help:
	@echo "HELP:"
	@echo "make assets"
	@echo "make vendor"

assets:
	go get -u github.com/jteeuwen/go-bindata/go-bindata
	cd server && go-bindata -pkg server data

vendor:
	GO15VENDOREXPERIMENT=1 godep save ./...

.PHONY: help assets vendor
