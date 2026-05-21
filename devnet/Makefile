.PHONY: dev clean-ready

SENTINEL := .devnet-ready
READY_TIMEOUT ?= 600

clean-ready:
	@rm -f $(SENTINEL)

dev: clean-ready
	go run -tags getaddress ../cmd/get_address.go
	tmuxp load -d ./devnet-session.yaml
	@echo "Waiting up to $(READY_TIMEOUT)s for devnet to become ready..."
	@i=0; while [ $$i -lt $(READY_TIMEOUT) ]; do \
	  if [ -f $(SENTINEL) ]; then \
	    echo "Devnet ready (after $${i}s)."; \
	    exec tmux a -t devnet; \
	  fi; \
	  i=$$((i+1)); sleep 1; \
	done; \
	echo "Devnet failed to become ready in $(READY_TIMEOUT)s" >&2; \
	tmux capture-pane -t devnet -p -S -200 || true; \
	exit 1
