.PHONY: build-node build-vendor build-web build-management management

build-node:
	go build -ldflags="-s -w" -trimpath -o bin/node ./cmd/dht-node

build-vendor:
	tar -xzf static/vendor.tar.gz -C static

build-web: build-vendor
	tsc -p static/tsconfig.json

build-management: build-web build-node
	go build -ldflags="-s -w" -trimpath -o bin/management ./cmd/management

management: build-management
	./bin/management
