package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

// Adversarial: cannot reach ACTIVE via MintRWA without locking bond ≥ MinIssuerBond.
func TestAdversarial_LowBondRejectedBeforeActive(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{
		Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "1",
	})
	require.ErrorIs(t, err, types.ErrBondTooLow)

	_, found := k.GetAsset(ctx, "gold-01")
	require.False(t, found)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero())

	// Mint/ACTIVE path is unreachable: attestation and mint both fail on missing asset.
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.ErrorIs(t, err, types.ErrAssetNotFound)

	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.ErrorIs(t, err, types.ErrAssetNotFound)
}

// Adversarial: SettleRWA must not release escrowed bond while outstanding supply remains.
func TestAdversarial_SettleRWACannotReclaimBondWithOutstandingSupply(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{
		Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000",
	})
	require.NoError(t, err)
	bondEscrowed := bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount
	require.Equal(t, math.NewInt(10_000_000_000), bondEscrowed)

	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{
		Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "400",
	})
	require.NoError(t, err)

	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.Error(t, err)

	rec, found := k.GetAsset(ctx, "gold-01")
	require.True(t, found)
	require.Equal(t, types.AssetStatus_ASSET_STATUS_ACTIVE, rec.Status)
	require.Equal(t, bondEscrowed, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)
}

// Adversarial: restricted rwa/* sends are blocked at the bank SendRestrictionFn chokepoint.
// authz.MsgExec, MsgMultiSend, and IBC transfer escrow all funnel through
// BankKeeper.SendCoins (Phase 3 D5), so this unit-level SendRestriction assertion
// covers those wrapper paths too.
func TestAdversarial_SendRestrictionBlocksDeniedAddress(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{
		Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000",
	})
	require.NoError(t, err)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	denied := sdkAcc(t)
	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{
		Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false, AddDeny: []string{denied.String()},
	})
	require.NoError(t, err)

	allowed := sdkAcc(t)
	require.NoError(t, k.AddAllowlist(ctx, "gold-01", issuer))
	require.NoError(t, k.AddAllowlist(ctx, "gold-01", allowed))

	_, err = k.SendRestriction(ctx, issuer, denied, sdk.NewCoins(sdk.NewCoin("rwa/gold-01", math.NewInt(1))))
	require.ErrorIs(t, err, types.ErrTransferRestricted)
}
