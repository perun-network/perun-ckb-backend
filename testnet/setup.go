package testnet

import (
	"fmt"
	"log"
	"os/exec"
	"time"
)

const (
	// DevnetDir is the directory where the devnet setup scripts are located.
	DevnetDir = "/devnet/"
)

var (
	ckbCmd   *exec.Cmd
	minerCmd *exec.Cmd
)

// RunSetupScript runs the setup script before starting the devnet
func RunSetupScript(root string) error {
	// Run the setup-devnet.sh script
	cmd := exec.Command("./setup-devnet.sh")
	cmd.Dir = root + DevnetDir
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run setup-devnet.sh: %v", err)
	}

	log.Println("Setup script executed successfully.")
	return nil
}

// StartDevnet starts the devnet by running the setup script and starting the CKB node and miner
func StartDevnet(root string) error {
	// Check if the devnet is already running
	if ckbCmd != nil || minerCmd != nil {
		err := StopDevnet()
		if err != nil {
			return fmt.Errorf("failed to stop devnet: %v", err)
		}
	}

	// Run the setup script first
	err := RunSetupScript(root)
	if err != nil {
		return fmt.Errorf("failed to run setup script: %v", err)
	}

	// Start CKB node in a Docker container
	ckbCmd = exec.Command("ckb", "run")
	ckbCmd.Dir = root + DevnetDir
	if err := ckbCmd.Start(); err != nil {
		return fmt.Errorf("failed to start CKB node: %v", err)
	}
	log.Printf("CKB node started with PID %d", ckbCmd.Process.Pid)

	time.Sleep(3 * time.Second) // Wait for the node to start

	// Start miner in the background within the CKB container
	minerCmd = exec.Command("ckb", "miner")
	minerCmd.Dir = root + DevnetDir
	if err := minerCmd.Start(); err != nil {
		return fmt.Errorf("failed to start CKB miner: %v", err)
	}
	log.Printf("CKB miner started with PID %d", minerCmd.Process.Pid)

	// Run the setup scripts inside the container
	printAccountsCmd := exec.Command("./print_accounts.sh")
	printAccountsCmd.Dir = root + DevnetDir
	if err := printAccountsCmd.Run(); err != nil {
		return fmt.Errorf("failed to run print_accounts.sh: %v", err)
	}

	time.Sleep(3 * time.Second)

	// Fund the accounts using the expect script
	fundAccountsCmd := exec.Command("expect", "fund_accounts.expect")
	fundAccountsCmd.Dir = root + DevnetDir
	if err := fundAccountsCmd.Run(); err != nil {
		return fmt.Errorf("failed to run fund_accounts.expect: %v", err)
	}

	log.Printf("Accounts funded successfully.")
	time.Sleep(4 * time.Second)
	deployContractsCmd := exec.Command("./deploy_contracts.sh")
	deployContractsCmd.Dir = root + DevnetDir
	// Start the script
	if err := deployContractsCmd.Run(); err != nil {
		return fmt.Errorf("failed to run deploy_contracts.sh: %v", err)
	}
	log.Printf("Contracts deployed successfully.")

	// Wait for 15 seconds before funding SUDTs
	time.Sleep(15 * time.Second)
	sudtFundCmd := exec.Command("./sudt_helper.sh", "fund")
	sudtFundCmd.Dir = root + DevnetDir
	if err := sudtFundCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute sudt_helper.sh fund: %v", err)
	}

	log.Printf("SUDTs funded successfully.")

	// List SUDT balances after 10 seconds
	time.Sleep(10 * time.Second)
	sudtBalancesCmd := exec.Command("./sudt_helper.sh", "balances")
	sudtBalancesCmd.Dir = root + DevnetDir
	if err := sudtBalancesCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute sudt_helper.sh balances: %v", err)
	}

	log.Printf("SUDT balances listed successfully.")
	time.Sleep(10 * time.Second)
	log.Println("Devnet started in background.")
	return nil
}

// StopDevnet stops the devnet by killing the CKB node and miner processes
func StopDevnet() error {
	if ckbCmd != nil {
		err := ckbCmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("failed to kill CKB node process: %v", err)
		}
		ckbCmd = nil
	}

	if minerCmd != nil {
		err := minerCmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("failed to kill CKB miner process: %v", err)
		}
		minerCmd = nil
	}
	log.Println("Devnet stopped.")
	return nil
}
