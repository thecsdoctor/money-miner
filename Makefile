# money-miner — root build orchestration (dossier 06).
# The worker matrix is pure Go (adapter pattern keeps CGO_ENABLED=0), so all
# six targets cross-compile from one machine.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BACKEND := money-miner-backend
FRONTEND := money-miner-frontend
DIST := dist

PLATFORMS := linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64

.PHONY: all build-worker build-server build-wasm test lint vet openapi-types release clean dev

all: build-server build-worker build-wasm ## build everything

build-worker: ## cross-compile the swarm worker for all 6 targets
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out=$(DIST)/money-miner-worker_$${os}_$${arch}; \
		[ "$$os" = "windows" ] && out=$$out.exe; \
		echo "building $$out"; \
		(cd $(BACKEND) && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" -o ../$$out ./cmd/worker); \
	done

build-server: ## build the master server for container arches
	@mkdir -p $(DIST)
	@for p in linux/amd64 linux/arm64; do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "building server $$os/$$arch"; \
		(cd $(BACKEND) && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" -o ../$(DIST)/money-miner-server_$${os}_$${arch} ./cmd/server); \
	done

build-wasm: ## build the browser hasher (GOOS=js GOARCH=wasm) into the frontend
	@mkdir -p $(FRONTEND)/public/wasm
	cd $(BACKEND) && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath \
		-ldflags "-s -w" -o ../$(FRONTEND)/public/wasm/browserhash.wasm ./cmd/browserhash
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(FRONTEND)/public/wasm/wasm_exec.js

test: ## go test ./... + frontend typecheck
	cd $(BACKEND) && go test ./...
	cd $(FRONTEND) && npx tsc --noEmit

vet: ## go vet
	cd $(BACKEND) && go vet ./...

lint: ## golangci-lint (backend) + tsc (frontend)
	cd $(BACKEND) && (command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed; go vet already covers the basics")
	cd $(FRONTEND) && npx tsc --noEmit

openapi-types: ## regenerate API types from the contract of record
	$(MAKE) -C money-miner-api types

release: clean all sha256sums ## full release build + checksum manifest

sha256sums: ## sha256sums.txt over dist/ (release asset)
	cd $(DIST) && sha256sum money-miner-worker_* money-miner-server_* > sha256sums.txt
	@echo "wrote $(DIST)/sha256sums.txt"

clean:
	rm -rf $(DIST)

dev: ## local bring-up: compose stack (deploy/) + swagger editor
	cd deploy && docker compose up -d --build
	cd money-miner-api && docker compose up -d
