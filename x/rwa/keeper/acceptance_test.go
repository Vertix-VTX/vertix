package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/testutil/keeper"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

// Spec §11: full lifecycle register → attest → mint → transfer → settle, with
// oracle gating, bond escrow/return, restriction enforcement, and fee routing.
func TestAcceptance_FullLifecycle(t *testing.T) {
	bank := keeper.NewMockBank()
	distr := keeper.NewMockDistribution()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1900)}, distr)
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	// register (bond locked)
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(10_000_000_000), bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)

	// attest (oracle gate + snapshot)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)

	// mint (fee to collector)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(1000), bank.GetBalance(ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), "uvtx").Amount)

	// transfer to bob then back (so issuer holds all units before settle)
	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "400000"})
	require.NoError(t, err)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: bob.String(), Recipient: issuer.String(), AssetId: "gold-01", Amount: "400000"})
	require.NoError(t, err)

	// settle (burn, bond returned, settle fee charged)
	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_SETTLED, rec.Status)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero()) // bond released

	// invariants hold at the end
	_, broken := rwakeeper.BondInvariant(k)(ctx)
	require.False(t, broken)
	_, broken = rwakeeper.DenomInvariant(k)(ctx)
	require.False(t, broken)
}

// Spec §11 / D5: bank send-restriction blocks restricted rwa/* sends (bypass test).
func TestAcceptance_SendRestrictionBlocksBypass(t *testing.T) {
	bank := keeper.NewMockBank()
	k, ctx := keeper.RWAKeeper(t, bank, keeper.MockOracle{Price: math.LegacyNewDec(1)}, keeper.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "G", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)
	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false})
	require.NoError(t, err)

	// a raw bank send (simulated via the registered restriction) is blocked
	bob := sdkAcc(t)
	_, err = k.SendRestriction(ctx, issuer, bob, sdk.NewCoins(sdk.NewCoin("rwa/gold-01", math.NewInt(1))))
	require.ErrorIs(t, err, types.ErrTransferRestricted)
}
