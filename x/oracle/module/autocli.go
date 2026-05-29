package oracle

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/vertix-network/vertix/x/oracle/types"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Price",
					Use:       "price [pair]",
					Short:     "Query the latest aggregated price for a pair",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pair"},
					},
				},
				{
					RpcMethod: "Twap",
					Use:       "twap [pair] [window-seconds]",
					Short:     "Query the time-weighted average price for a pair and window",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pair"},
						{ProtoField: "window_seconds"},
					},
				},
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Query the current oracle module parameters",
				},
				{
					RpcMethod: "MissCounter",
					Use:       "miss-counter [validator]",
					Short:     "Query miss statistics for a validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "validator"},
					},
				},
				{
					RpcMethod: "Feeder",
					Use:       "feeder [validator]",
					Short:     "Query the feeder account delegated by a validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "validator"},
					},
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true,
				},
				{
					RpcMethod: "SetFeeder",
					Use:       "set-feeder [feeder]",
					Short:     "Delegate a feeder account for the validator operator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "feeder"},
					},
				},
				{
					RpcMethod: "SubmitFeed",
					Use:       "submit-feed [validator] [pair] [price]",
					Short:     "Submit a price feed for a pair (feeder-signed)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "validator"},
						{ProtoField: "pair"},
						{ProtoField: "price"},
					},
				},
			},
		},
	}
}
