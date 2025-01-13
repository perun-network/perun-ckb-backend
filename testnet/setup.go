package testnet

import (
	"fmt"
	"log"
	"os/exec"
	"time"
)

const (
	// DevnetDir is the directory where the devnet setup scripts are located.
	DevnetDir = "./devnet/"
)

var (
	ckbCmd             *exec.Cmd
	minerCmd           *exec.Cmd
	printAccountsCmd   *exec.Cmd
	fundAccountsCmd    *exec.Cmd
	deployContractsCmd *exec.Cmd
	sudtFundCmd        *exec.Cmd
	sudtBalancesCmd    *exec.Cmd
)

// RunSetupScript runs the setup script before starting the devnet
func RunSetupScript() error {
	// Run the setup-devnet.sh script
	cmd := exec.Command("./setup-devnet.sh")
	cmd.Dir = DevnetDir
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run setup-devnet.sh: %v", err)
	}
	log.Println("Setup script executed successfully.")
	return nil
}

// StartDevnet starts the devnet by running the setup script and starting the CKB node and miner
func StartDevnet() error {
	// Run the setup script first
	err := RunSetupScript()
	if err != nil {
		return fmt.Errorf("failed to run setup script: %v", err)
	}

	// Start CKB node in a Docker container
	ckbCmd = exec.Command("ckb", "run")
	ckbCmd.Dir = DevnetDir
	if err := ckbCmd.Start(); err != nil {
		return fmt.Errorf("failed to start CKB node: %v", err)
	}
	log.Printf("CKB node started with PID %d", ckbCmd.Process.Pid)

	time.Sleep(3 * time.Second) // Wait for the node to start

	// Start miner in the background within the CKB container
	minerCmd = exec.Command("ckb", "miner")
	minerCmd.Dir = DevnetDir
	if err := minerCmd.Start(); err != nil {
		return fmt.Errorf("failed to start CKB miner: %v", err)
	}
	log.Printf("CKB miner started with PID %d", minerCmd.Process.Pid)

	// Run the setup scripts inside the container
	printAccountsCmd := exec.Command("./print_accounts.sh")
	printAccountsCmd.Dir = DevnetDir
	if err := printAccountsCmd.Start(); err != nil {
		return fmt.Errorf("failed to run print_accounts.sh: %v", err)
	}

	time.Sleep(6 * time.Second)

	// Fund the accounts using the expect script
	fundAccountsCmd = exec.Command("expect", "fund_accounts.expect")
	fundAccountsCmd.Dir = DevnetDir
	if err := fundAccountsCmd.Start(); err != nil {
		return fmt.Errorf("failed to run fund_accounts.expect: %v", err)
	}

	// Wait for the script to complete
	if err := fundAccountsCmd.Wait(); err != nil {
		return fmt.Errorf("failed to wait for fund_accounts.expect: %v", err)
	}

	log.Printf("Accounts funded successfully.")

	// Wait a bit before deploying contracts
	time.Sleep(10 * time.Second)
	deployContractsCmd = exec.Command("./deploy_contracts.sh")
	deployContractsCmd.Dir = DevnetDir
	// Start the script
	if err := deployContractsCmd.Start(); err != nil {
		return fmt.Errorf("failed to run deploy_contracts.sh: %v", err)
	}

	// Wait for script completion
	if err := deployContractsCmd.Wait(); err != nil {
		return fmt.Errorf("deploy_contracts.sh execution failed: %v", err)
	}
	log.Printf("Contracts deployed successfully.")

	// Wait for 15 seconds before funding SUDTs
	time.Sleep(15 * time.Second)
	sudtFundCmd = exec.Command("./sudt_helper.sh", "fund")
	sudtFundCmd.Dir = DevnetDir
	if err := sudtFundCmd.Start(); err != nil {
		return fmt.Errorf("failed to execute sudt_helper.sh fund: %v", err)
	}

	// Wait for the command to complete
	if err := sudtFundCmd.Wait(); err != nil {
		return fmt.Errorf("sudt_helper.sh fund encountered an error: %v", err)
	}

	log.Printf("SUDTs funded successfully.")

	// List SUDT balances after 10 seconds
	time.Sleep(10 * time.Second)
	sudtBalancesCmd = exec.Command("./sudt_helper.sh", "balances")
	sudtBalancesCmd.Dir = DevnetDir
	if err := sudtBalancesCmd.Start(); err != nil {
		return fmt.Errorf("failed to execute sudt_helper.sh balances: %v", err)
	}

	// Wait for the command to complete
	if err := sudtBalancesCmd.Wait(); err != nil {
		return fmt.Errorf("sudt_helper.sh balances encountered an error: %v", err)
	}

	log.Printf("SUDT balances listed successfully.")

	log.Println("Devnet started in background.")
	return nil
}

// StopDevnet stops the devnet by killing the CKB node and miner processes
func StopDevnet() error {
	err := ckbCmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill CKB node process: %v", err)
	}

	err = minerCmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill CKB miner process: %v", err)
	}

	log.Println("Devnet stopped.")
	return nil
}
