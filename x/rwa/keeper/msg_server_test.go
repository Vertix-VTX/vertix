package keeper_test

import (
	"context"
	"errors"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	rwakeeper "github.com/vertix-network/vertix/x/rwa/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

var errFakeStale = errors.New("oracle: stale price")

func setupServer(t *testing.T, oracle types.OracleKeeper) (types.MsgServer, rwakeeper.Keeper, *keepertest.MockBank, sdk.Context) {
	bank := keepertest.NewMockBank()
	k, ctx := keepertest.RWAKeeper(t, bank, oracle, keepertest.NewMockDistribution())
	return rwakeeper.NewMsgServerImpl(k), k, bank, ctx
}

func setupMsgServer(t testing.TB) (rwakeeper.Keeper, types.MsgServer, context.Context) {
	k, ctx := keepertest.RwaKeeper(t)
	return k, rwakeeper.NewMsgServerImpl(k), ctx
}

func TestRegisterAsset(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{
		Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000",
	})
	require.NoError(t, err)

	rec, found := k.GetAsset(ctx, "gold-01")
	require.True(t, found)
	require.Equal(t, types.AssetStatus_ASSET_STATUS_DRAFT, rec.Status)
	require.Equal(t, "rwa/gold-01", rec.Denom)
	require.True(t, rec.AllowAll)
	require.Equal(t, math.NewInt(10_000_000_000), bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").Amount)
}

func TestRegisterAssetRejectsDuplicateAndLowBond(t *testing.T) {
	srv, _, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))

	base := &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "Gold", OraclePair: "XAU:USD", Bond: "10000000000"}
	_, err := srv.RegisterAsset(ctx, base)
	require.NoError(t, err)

	_, err = srv.RegisterAsset(ctx, base)
	require.ErrorIs(t, err, types.ErrAssetExists)

	_, err = srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "silver-01", Name: "Silver", OraclePair: "XAG:USD", Bond: "1"})
	require.ErrorIs(t, err, types.ErrBondTooLow)
}

func registerDraft(t *testing.T, srv types.MsgServer, _ rwakeeper.Keeper, bank *keepertest.MockBank, ctx sdk.Context, id string) sdk.AccAddress {
	t.Helper()
	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: id, Name: "N", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	return issuer
}

func TestAttestAssetSuccess(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1900)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")

	_, err := srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_ATTESTED, rec.Status)
	require.Equal(t, "1900.000000000000000000", rec.AttestedPrice)
}

func TestAttestAssetOracleFailure(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Err: errFakeStale})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")

	_, err := srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.ErrorIs(t, err, types.ErrAttestationFailed)
}

func attestAsset(t *testing.T, srv types.MsgServer, issuer sdk.AccAddress, id string, ctx sdk.Context) {
	t.Helper()
	_, err := srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: id})
	require.NoError(t, err)
}

func TestMintRWA(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)

	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_ACTIVE, rec.Status)
	require.Equal(t, math.NewInt(1000000).String(), rec.NotionalMinted)
	require.Equal(t, math.NewInt(1000000), bank.GetBalance(ctx, issuer, "rwa/gold-01").Amount)
	feeCollector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	require.Equal(t, math.NewInt(1000), bank.GetBalance(ctx, feeCollector, "uvtx").Amount)
}

func TestMintRWARequiresAttested(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	_ = k
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.ErrorIs(t, err, types.ErrInvalidStatus)
}

func TestMintRWAReMintAccumulates(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)

	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "500000"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, math.NewInt(1500000).String(), rec.NotionalMinted)
	require.Equal(t, math.NewInt(1500000), bank.GetBalance(ctx, issuer, "rwa/gold-01").Amount)
}

func TestTransferRWA(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "300"})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(300), bank.GetBalance(ctx, bob, "rwa/gold-01").Amount)
	require.Equal(t, math.NewInt(700), bank.GetBalance(ctx, issuer, "rwa/gold-01").Amount)
}

func TestTransferRWARestricted(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false})
	require.NoError(t, err)

	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "100"})
	require.ErrorIs(t, err, types.ErrTransferRestricted)
}

func TestUpdateRestrictions(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")

	alice := sdkAcc(t)
	_, err := srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{
		Issuer: issuer.String(), AssetId: "gold-01", AllowAll: false, AddAllow: []string{alice.String()},
	})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.False(t, rec.AllowAll)
	require.True(t, k.IsAllowlisted(ctx, "gold-01", alice))

	_, err = srv.UpdateRestrictions(ctx, &types.MsgUpdateRestrictions{Issuer: sdkAcc(t).String(), AssetId: "gold-01", AllowAll: true})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestSettleRWA(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000000"})
	require.NoError(t, err)

	issuerUvtxBefore := bank.GetBalance(ctx, issuer, "uvtx").Amount

	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)

	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_SETTLED, rec.Status)
	require.True(t, bank.GetBalance(ctx, issuer, "rwa/gold-01").IsZero())
	expected := issuerUvtxBefore.Add(math.NewInt(10_000_000_000)).Sub(math.NewInt(1000))
	require.Equal(t, expected, bank.GetBalance(ctx, issuer, "uvtx").Amount)
	require.True(t, bank.GetBalance(ctx, k.ModuleAddress(), "uvtx").IsZero())
}

func TestSettleRWARequiresIssuerHoldsAllUnits(t *testing.T) {
	srv, k, bank, ctx := setupServer(t, keepertest.MockOracle{Price: math.LegacyNewDec(1)})
	issuer := registerDraft(t, srv, k, bank, ctx, "gold-01")
	attestAsset(t, srv, issuer, "gold-01", ctx)
	_, err := srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)
	bob := sdkAcc(t)
	_, err = srv.TransferRWA(ctx, &types.MsgTransferRWA{Sender: issuer.String(), Recipient: bob.String(), AssetId: "gold-01", Amount: "400"})
	require.NoError(t, err)
	_, err = srv.SettleRWA(ctx, &types.MsgSettleRWA{Issuer: issuer.String(), AssetId: "gold-01"})
	require.Error(t, err)
	_ = k
}

func TestSlashBond(t *testing.T) {
	bank := keepertest.NewMockBank()
	distr := keepertest.NewMockDistribution()
	k, ctx := keepertest.RWAKeeper(t, bank, keepertest.MockOracle{Price: math.LegacyNewDec(1)}, distr)
	srv := rwakeeper.NewMsgServerImpl(k)

	issuer := sdkAcc(t)
	bank.SetBalance(issuer, sdk.NewCoins(sdk.NewCoin("uvtx", math.NewInt(20_000_000_000))))
	_, err := srv.RegisterAsset(ctx, &types.MsgRegisterAsset{Issuer: issuer.String(), AssetId: "gold-01", Name: "N", OraclePair: "XAU:USD", Bond: "10000000000"})
	require.NoError(t, err)
	_, err = srv.AttestAsset(ctx, &types.MsgAttestAsset{Issuer: issuer.String(), AssetId: "gold-01"})
	require.NoError(t, err)
	_, err = srv.MintRWA(ctx, &types.MsgMintRWA{Issuer: issuer.String(), AssetId: "gold-01", Notional: "1000"})
	require.NoError(t, err)

	_, err = srv.SlashBond(ctx, &types.MsgSlashBond{Authority: sdkAcc(t).String(), AssetId: "gold-01"})
	require.ErrorIs(t, err, types.ErrUnauthorized)

	_, err = srv.SlashBond(ctx, &types.MsgSlashBond{Authority: k.GetAuthority(), AssetId: "gold-01", Reason: "fraud"})
	require.NoError(t, err)
	rec, _ := k.GetAsset(ctx, "gold-01")
	require.Equal(t, types.AssetStatus_ASSET_STATUS_SETTLED, rec.Status)
	require.Equal(t, math.NewInt(10_000_000_000), distr.Funded.AmountOf("uvtx"))
}

func TestUpdateParams(t *testing.T) {
	k, ctx := keepertest.RWAKeeper(t, keepertest.NewMockBank(), keepertest.MockOracle{Price: math.LegacyNewDec(1)}, keepertest.NewMockDistribution())
	srv := rwakeeper.NewMsgServerImpl(k)

	newParams := types.RWAParams{MinIssuerBond: "5000000000", MintFeeRate: "0.002", SettleFeeRate: "0.002"}
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{Authority: k.GetAuthority(), Params: newParams})
	require.NoError(t, err)
	got, _ := k.GetParams(ctx)
	require.Equal(t, newParams, got)

	_, err = srv.UpdateParams(ctx, &types.MsgUpdateParams{Authority: sdkAcc(t).String(), Params: types.DefaultParams()})
	require.ErrorIs(t, err, types.ErrInvalidSigner)
}
