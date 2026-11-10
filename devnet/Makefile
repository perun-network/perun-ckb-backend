.PHONY: dev

dev:
	go run ../cmd/get_address.go
	tmuxp load -d ./devnet-session.yaml
	@echo "Waiting for the local devnet to start..."
	@sleep 60
	go run ../cmd/fund_sudt.go
	tmux a -t devnet

# Run after the existing devnet has been deployed with PERUN_XUDT_TESTS=1.
.PHONY: test-xudt
test-xudt:
	cd .. && go test -tags xudt_integration ./client -run '^TestXUDT' -count=1 -timeout 15m
