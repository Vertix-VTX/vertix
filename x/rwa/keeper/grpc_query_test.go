package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	keepertest "github.com/vertix-network/vertix/testutil/keeper"
	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestQueries(t *testing.T) {
	bank := keepertest.NewMockBank()
	k, ctx := keepertest.RWAKeeper(t, bank, keepertest.MockOracle{Price: math.LegacyNewDec(1)}, keepertest.NewMockDistribution())
	issuer := sdkAcc(t)
	require.NoError(t, k.SetAsset(ctx, types.AssetRecord{AssetId: "gold", Issuer: issuer.String(), Status: types.AssetStatus_ASSET_STATUS_DRAFT, Denom: "rwa/gold", Bond: "1", NotionalMinted: "0", AllowAll: false}))
	alice := sdkAcc(t)
	require.NoError(t, k.AddAllowlist(ctx, "gold", alice))

	asset, err := k.Asset(ctx, &types.QueryAssetRequest{AssetId: "gold"})
	require.NoError(t, err)
	require.Equal(t, "rwa/gold", asset.Asset.Denom)

	_, err = k.Asset(ctx, &types.QueryAssetRequest{AssetId: "missing"})
	require.Error(t, err)

	byIssuer, err := k.AssetsByIssuer(ctx, &types.QueryAssetsByIssuerRequest{Issuer: issuer.String()})
	require.NoError(t, err)
	require.Len(t, byIssuer.Assets, 1)

	restr, err := k.Restrictions(ctx, &types.QueryRestrictionsRequest{AssetId: "gold"})
	require.NoError(t, err)
	require.False(t, restr.AllowAll)
	require.Contains(t, restr.Allowlist, alice.String())

	params, err := k.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), params.Params)
}
