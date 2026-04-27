.PHONY: dev

dev:
	go run ../cmd/get_address.go
	tmuxp load -d ./devnet-session.yaml
	@echo "Waiting for the local devnet to start..."
	@sleep 60
	go run ../cmd/fund_sudt.go
	tmux a -t devnet
