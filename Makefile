.PHONY: hooks up down logs test check verify

COMPOSE = docker compose -f deploy/compose.yaml -f deploy/compose.ports.yaml

hooks: ## Switch on the commit check in this clone
	@git config core.hooksPath .githooks

up: ## Start everything
	$(COMPOSE) up -d --build --wait

down: ## Stop everything
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

test check: hooks ## Fast checks
	./scripts/check

verify: hooks ## Full suite, on a throwaway copy of the system
	./scripts/verify
