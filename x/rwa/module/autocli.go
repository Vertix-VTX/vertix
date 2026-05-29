package rwa

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "Asset", Use: "asset [asset-id]", Short: "Query an asset record", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "AssetsByIssuer", Use: "assets-by-issuer [issuer]", Short: "Query assets by issuer", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "issuer"}}},
				{RpcMethod: "Restrictions", Use: "restrictions [asset-id]", Short: "Query transfer restrictions", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "Params", Use: "params", Short: "Query module params"},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "RegisterAsset", Use: "register-asset [asset-id] [name] [oracle-pair] [bond]", Short: "Register a new RWA asset", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}, {ProtoField: "name"}, {ProtoField: "oracle_pair"}, {ProtoField: "bond"}}},
				{RpcMethod: "AttestAsset", Use: "attest-asset [asset-id]", Short: "Attest an asset against its oracle price", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "MintRWA", Use: "mint-rwa [asset-id] [notional]", Short: "Mint rwa/{id} tokens", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}, {ProtoField: "notional"}}},
				{RpcMethod: "TransferRWA", Use: "transfer-rwa [recipient] [asset-id] [amount]", Short: "Transfer rwa/{id} tokens", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "recipient"}, {ProtoField: "asset_id"}, {ProtoField: "amount"}}},
				{RpcMethod: "SettleRWA", Use: "settle-rwa [asset-id]", Short: "Settle an asset and reclaim bond", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "asset_id"}}},
				{RpcMethod: "UpdateRestrictions", Skip: true},
				{RpcMethod: "SlashBond", Skip: true},
				{RpcMethod: "UpdateParams", Skip: true},
			},
		},
	}
}
