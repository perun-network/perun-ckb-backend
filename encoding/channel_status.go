package encoding

import "github.com/nervosnetwork/ckb-sdk-go/v2/types/molecule"

func ToFundedChannelStatus(status molecule.ChannelStatus) molecule.ChannelStatus {
	statusBuilder := status.AsBuilder()
	return statusBuilder.Funded(True).Build()
}
