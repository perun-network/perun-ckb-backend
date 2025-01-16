package testnet_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"perun.network/perun-ckb-backend/testnet"
)

func TestRunSetupScript(t *testing.T) {
	err := testnet.RunSetupScript(".")
	require.NoError(t, err, "failed to run setup script")
}

func TestSetup(t *testing.T) {
	err := testnet.StartDevnet(".")
	require.NoError(t, err, "failed to start devnet")

	err = testnet.StopDevnet()
	require.NoError(t, err, "failed to stop devnet")
}
