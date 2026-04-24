.PHONY: dev

dev:
	go run ../cmd/get_address.go
	tmuxp load -d ./devnet-session.yaml
	sleep 50
	go run ../cmd/fund_sudt.go
