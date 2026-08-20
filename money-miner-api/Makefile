# money-miner-api — OpenAPI contract of record + type generation.
#
# `make types` regenerates:
#   ../money-miner-frontend/src/api/types.generated.ts   (openapi-typescript)
#   ../money-miner-backend/internal/apigen/              (oapi-codegen)
# Generated files are gitignored; the backend and frontend keep their own
# hand-written domain types, so a generator version bump can never break a
# tagged build. This target is a developer convenience.

SPEC := openapi.yaml
BACKEND_GEN := ../money-miner-backend/internal/apigen
FRONTEND_GEN := ../money-miner-frontend/src/api/types.generated.ts

.PHONY: types validate editor

# Node comes via nvm when present, otherwise the system node is used.
NODE_ENV = (. ~/.nvm/nvm.sh 2>/dev/null || true)

types: ## Generate TS + Go types from the OpenAPI spec
	$(NODE_ENV); npx --yes openapi-typescript $(SPEC) -o $(FRONTEND_GEN)
	mkdir -p $(BACKEND_GEN)
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
		-generate types,client -package apigen $(SPEC) > $(BACKEND_GEN)/types.gen.go

validate: ## Validate the spec (swagger-cli)
	$(NODE_ENV); npx --yes @apidevtools/swagger-cli@4.0.4 validate $(SPEC)

editor: ## Run swagger-editor on 127.0.0.1:3301
	docker compose up -d
	@echo "open http://127.0.0.1:3301/?url=/openapi.yaml"
