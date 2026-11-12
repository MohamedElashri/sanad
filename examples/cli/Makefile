SANAD ?= sanad
SANAD_BASE ?= origin/main

.PHONY: sanad-check sanad-local sanad-plan
sanad-check:
	$(SANAD) check --base "$(SANAD_BASE)"

sanad-local:
	$(SANAD) check --offline --coverage-only

sanad-plan:
	$(SANAD) plan
