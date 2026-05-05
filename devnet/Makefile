.PHONY: dev

dev:
	go run ../cmd/get_address.go
	tmuxp load -d ./devnet-session.yaml
	@echo "Waiting for the local devnet to start..."
	@sleep 60
	tmux a -t devnet
