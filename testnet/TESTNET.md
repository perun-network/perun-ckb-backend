# Perun CKB Integration Test on Testnet

This documents serves as tutorial to execute the integration test between [Perun-CKB-Contracts] and [Perun-CKB-Backend] on the Testnet.

## Requirements
Install [ckb-cli](https://docs.nervos.org/docs/sdk-and-devtool/ckb-cli) to generate accounts and fetch transactions details from the testnet. 

## Testnet Contracts
The Perun-CKB-Contracts are currently live on [CKB-Testnet](https://testnet.explorer.nervos.org/). 
The transactions are given here:
```json
{
  "cell_recipes": [
    {
      "name": "sudt",
      "tx_hash": "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
      "index": 0,
      "occupied_capacity": 3437400000000,
      "data_hash": "0xb875ff254fcaee9c5e164f3f2bf02f8e10a0d00526db46571ff22abae0766f11",
      "type_id": "0xd7cb2e882ae04f0ba2d00d46d49ae2a7375f0e0d0a5d0d4aa48cef428d5bc5e5"
    },
    {
      "name": "pcts",
      "tx_hash": "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
      "index": 1,
      "occupied_capacity": 32056600000000,
      "data_hash": "0x9c8eb7243aef83b0135d450407292da728753870b804a1d64a27d901f7b9640f",
      "type_id": "0x96b5e79709e3c4931a35e5af67356e4ab752e5a990fce241fa17c4f6c3d510e2"
    },
    {
      "name": "pcls",
      "tx_hash": "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
      "index": 2,
      "occupied_capacity": 6277400000000,
      "data_hash": "0x358519445ce23f8befc6580c5359c3477b8b5283f397a29806f0e95522a6adb8",
      "type_id": "0x4fa6fd8c0ae0e4b870ed748f86cc42afcb47380f51a6864852820c127acb8f83"
    },
    {
      "name": "pfls",
      "tx_hash": "0xc247df0052ab5d67b6da04bf6f0743696a83db0cf94e2fef192cd29ef4cfe799",
      "index": 3,
      "occupied_capacity": 4834200000000,
      "data_hash": "0xd0507f41f9ddc3ef784e3a1c561d7d6e2dd08f0b24f9c9d61b9b93747d4a8295",
      "type_id": "0xa8690a18bde4123fa04e7e5823f0554f196ec0bd04f3bbf8ed4360902fed05a9"
    }
  ],
  "dep_group_recipes": []
}
```

## Run the Testnet Integration Test
1. Generate test accounts:
```bash
# Generate accounts for Alice, Bob, Ingrid
cd testnet/
ckb-cli account new >  accounts/alice.txt
ckb-cli account new >  accounts/bob.txt
ckb-cli account new >  accounts/ingrid.txt

# Fetch the private key to be used in 
ckb-cli account export --lock-arg <alice-lock-args> --extended-privkey-path ./accounts/alice.pk
ckb-cli account export --lock-arg <bob-lock-args> --extended-privkey-path ./accounts/bob.pk
ckb-cli account export --lock-arg <ingrid-lock-args> --extended-privkey-path ./accounts/ingrid.pk

cd ..
```
2. Fund the `accounts` with at least 200CKB using [Testnet Faucet](https://faucet.nervos.org/)
3. Export the private key of each accounts to environment variables.
```sh
export ALICE_PK="0xabc123..."
export BOB_PK="0xdef456..."
export INGRID_PK="0x7890..."
```
4. Run the integration test
```bash
go test ./... -testnet
```
