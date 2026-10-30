package transaction

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func (ptb *PerunTransactionBuilder) xudtOutputData(output *types.CellOutput, data []byte) ([]byte, error) {
	if output.Type == nil {
		return data, nil
	}
	config, ok := ptb.xudts[output.Type.Hash()]
	if !ok {
		return data, nil
	}
	if len(data) < 16 {
		return nil, fmt.Errorf("xUDT output is missing its amount")
	}
	tail := data[16:]
	if config.OutputData != nil {
		be := make([]byte, 16)
		for i := range be {
			be[15-i] = data[i]
		}
		var err error
		tail, err = config.OutputData(*output, new(big.Int).SetBytes(be))
		if err != nil {
			return nil, fmt.Errorf("xUDT output data: %w", err)
		}
	}
	if err := config.ValidateData(tail); err != nil {
		return nil, err
	}
	result := append([]byte(nil), data[:16]...)
	return append(result, tail...), nil
}

func (ptb *PerunTransactionBuilder) prepareXUDTOutputs() error {
	if len(ptb.Outputs) != len(ptb.OutputsData) {
		return fmt.Errorf("output/data count mismatch")
	}
	for i, output := range ptb.Outputs {
		if output == nil {
			return fmt.Errorf("nil output %d", i)
		}
		data, err := ptb.xudtOutputData(output, ptb.OutputsData[i])
		if err != nil {
			return fmt.Errorf("output %d: %w", i, err)
		}
		if output.Type != nil {
			if _, ok := ptb.xudts[output.Type.Hash()]; ok && output.Capacity < output.OccupiedCapacity(data) {
				return fmt.Errorf("xUDT output %d capacity is below occupied capacity; increase the channel asset MaxCapacity", i)
			}
		}
		ptb.OutputsData[i] = data
	}
	return nil
}

func (ptb *PerunTransactionBuilder) validateXUDTInput(output *types.CellOutput, data []byte) error {
	if output.Type == nil {
		return nil
	}
	config, ok := ptb.xudts[output.Type.Hash()]
	if !ok {
		return nil
	}
	if len(data) < 16 {
		return fmt.Errorf("xUDT input is missing its amount")
	}
	if err := config.ValidateData(data[16:]); err != nil {
		return err
	}
	if len(data) > 16 && config.OutputData == nil {
		return fmt.Errorf("xUDT input contains metadata: an output data provider is required to preserve its semantics")
	}
	return nil
}

func (ptb *PerunTransactionBuilder) prepareXUDTWitnesses() error {
	groups := ptb.CopyValidScriptGroups()
	// Proofs may commit to the transaction hash. All groups' dependencies must
	// therefore be present before invoking any witness provider.
	for _, group := range groups {
		if group.GroupType != types.ScriptTypeType {
			continue
		}
		if config, ok := ptb.xudts[group.Script.Hash()]; ok {
			for i := range config.CellDeps {
				ptb.AddCellDep(&config.CellDeps[i])
			}
		}
	}
	for _, group := range groups {
		if group.GroupType != types.ScriptTypeType {
			continue
		}
		config, ok := ptb.xudts[group.Script.Hash()]
		if !ok {
			continue
		}
		mode, err := config.Mode()
		if err != nil {
			return err
		}
		if mode == 0 {
			continue
		}
		data := make([][]byte, len(config.ExtensionScripts))
		if config.WitnessData != nil {
			data, err = config.WitnessData(ptb.BuildTransaction().TxView, append([]uint32(nil), group.InputIndices...), append([]uint32(nil), group.OutputIndices...))
			if err != nil {
				return fmt.Errorf("xUDT extension witness: %w", err)
			}
		}
		encoded, err := config.EncodeWitness(data)
		if err != nil {
			return err
		}
		var index uint32
		field := types.WitnessTypeInputType
		if len(group.InputIndices) > 0 {
			index = group.InputIndices[0]
		} else {
			index = group.OutputIndices[0]
			field = types.WitnessTypeOutputType
		}
		for uint32(len(ptb.Witnesses)) <= index {
			ptb.Witnesses = append(ptb.Witnesses, nil)
		}
		// Preserve the lock field and refuse to overwrite another protocol's data.
		if len(ptb.Witnesses[index]) > 0 {
			args, err := types.DeserializeWitnessArgs(ptb.Witnesses[index])
			if err != nil {
				return fmt.Errorf("xUDT witness %d: %w", index, err)
			}
			existing := args.InputType
			if field == types.WitnessTypeOutputType {
				existing = args.OutputType
			}
			if len(existing) > 0 && !bytes.Equal(existing, encoded) {
				return fmt.Errorf("xUDT witness field is already occupied")
			}
		}
		if err := ptb.SetWitness(uint(index), field, encoded); err != nil {
			return err
		}
	}
	return nil
}
