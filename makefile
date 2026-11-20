.PHONY: build-node build-web build-management management

build-node:
	go build -o bin/node ./cmd/dht-node

build-web:
	tsc -p static/tsconfig.json

build-management: build-web build-node
	go build -o bin/management ./cmd/management

management: build-management
	./bin/management
