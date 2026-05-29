package keeper

import (
	"context"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/vertix-network/vertix/x/rwa/types"
)

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = (*msgServer)(nil)

func (ms msgServer) RegisterAsset(ctx context.Context, msg *types.MsgRegisterAsset) (*types.MsgRegisterAssetResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if _, found := ms.GetAsset(ctx, msg.AssetId); found {
		return nil, types.ErrAssetExists.Wrap(msg.AssetId)
	}
	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	minBond, _ := params.MinIssuerBondInt()
	bond, ok := math.NewIntFromString(msg.Bond)
	if !ok {
		return nil, types.ErrInvalidAmount.Wrap("bond")
	}
	if bond.LT(minBond) {
		return nil, types.ErrBondTooLow.Wrapf("bond %s < min %s", bond, minBond)
	}
	issuer, err := sdk.AccAddressFromBech32(msg.Issuer)
	if err != nil {
		return nil, err
	}
	if err := ms.LockBond(ctx, issuer, bond); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec := types.AssetRecord{
		AssetId:        msg.AssetId,
		Issuer:         msg.Issuer,
		Name:           msg.Name,
		Description:    msg.Description,
		Status:         types.AssetStatus_ASSET_STATUS_DRAFT,
		OraclePair:     msg.OraclePair,
		Denom:          BuildDenom(msg.AssetId),
		Bond:           bond.String(),
		NotionalMinted: math.ZeroInt().String(),
		AllowAll:       true,
		CreatedAt:      sdkCtx.BlockTime(),
	}
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeAssetRegistered,
		sdk.NewAttribute(types.AttributeKeyAssetID, msg.AssetId),
		sdk.NewAttribute(types.AttributeKeyIssuer, msg.Issuer),
		sdk.NewAttribute(types.AttributeKeyBond, bond.String()),
	))
	return &types.MsgRegisterAssetResponse{}, nil
}

// requireIssuer loads an asset and asserts the caller is its issuer.
func (ms msgServer) requireIssuer(ctx context.Context, assetID, caller string) (types.AssetRecord, error) {
	rec, found := ms.GetAsset(ctx, assetID)
	if !found {
		return types.AssetRecord{}, types.ErrAssetNotFound.Wrap(assetID)
	}
	if rec.Issuer != caller {
		return types.AssetRecord{}, errorsmod.Wrapf(types.ErrUnauthorized, "caller %s is not issuer %s", caller, rec.Issuer)
	}
	return rec, nil
}

func (ms msgServer) AttestAsset(ctx context.Context, msg *types.MsgAttestAsset) (*types.MsgAttestAssetResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_DRAFT {
		return nil, types.ErrInvalidStatus.Wrapf("attest requires DRAFT, got %s", rec.Status)
	}
	price, err := ms.oracleKeeper.GetPrice(ctx, rec.OraclePair)
	if err != nil {
		return nil, types.ErrAttestationFailed.Wrapf("pair %s: %v", rec.OraclePair, err)
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.AssetStatus_ASSET_STATUS_ATTESTED
	rec.AttestedPrice = price.String()
	rec.AttestedAt = sdkCtx.BlockTime()
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeAssetAttested,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyOraclePair, rec.OraclePair),
		sdk.NewAttribute(types.AttributeKeyAttestedPrice, rec.AttestedPrice),
	))
	return &types.MsgAttestAssetResponse{}, nil
}

func (ms msgServer) MintRWA(ctx context.Context, msg *types.MsgMintRWA) (*types.MsgMintRWAResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	// First mint requires ATTESTED; subsequent mints accumulate on ACTIVE.
	if rec.Status != types.AssetStatus_ASSET_STATUS_ATTESTED && rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("mint requires ATTESTED or ACTIVE, got %s", rec.Status)
	}
	notional, ok := math.NewIntFromString(msg.Notional)
	if !ok || !notional.IsPositive() {
		return nil, types.ErrInvalidNotional.Wrap(msg.Notional)
	}
	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	rate, err := params.MintFeeRateDec()
	if err != nil {
		return nil, err
	}
	issuer, err := sdk.AccAddressFromBech32(msg.Issuer)
	if err != nil {
		return nil, err
	}
	fee := ms.ComputeFee(notional, rate)
	if err := ms.CollectFee(ctx, issuer, fee); err != nil {
		return nil, err
	}
	mintCoins := sdk.NewCoins(sdk.NewCoin(rec.Denom, notional))
	if err := ms.bankKeeper.MintCoins(ctx, types.ModuleName, mintCoins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, issuer, mintCoins); err != nil {
		return nil, err
	}
	prev, _ := math.NewIntFromString(rec.NotionalMinted)
	rec.NotionalMinted = prev.Add(notional).String()
	rec.Status = types.AssetStatus_ASSET_STATUS_ACTIVE
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRWAMinted,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyNotional, notional.String()),
		sdk.NewAttribute(types.AttributeKeyMintFee, fee.String()),
	))
	return &types.MsgMintRWAResponse{}, nil
}

func (ms msgServer) TransferRWA(ctx context.Context, msg *types.MsgTransferRWA) (*types.MsgTransferRWAResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, found := ms.GetAsset(ctx, msg.AssetId)
	if !found {
		return nil, types.ErrAssetNotFound.Wrap(msg.AssetId)
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("transfer requires ACTIVE, got %s", rec.Status)
	}
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, err
	}
	recipient, err := sdk.AccAddressFromBech32(msg.Recipient)
	if err != nil {
		return nil, err
	}
	if err := ms.CheckTransferAllowed(ctx, msg.AssetId, sender, recipient); err != nil {
		return nil, err
	}
	amount, ok := math.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, types.ErrInvalidAmount.Wrap(msg.Amount)
	}
	coins := sdk.NewCoins(sdk.NewCoin(rec.Denom, amount))
	if err := ms.bankKeeper.SendCoins(ctx, sender, recipient, coins); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRWATransferred,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeySender, msg.Sender),
		sdk.NewAttribute(types.AttributeKeyRecipient, msg.Recipient),
		sdk.NewAttribute(types.AttributeKeyAmount, amount.String()),
	))
	return &types.MsgTransferRWAResponse{}, nil
}

func (ms msgServer) UpdateRestrictions(ctx context.Context, msg *types.MsgUpdateRestrictions) (*types.MsgUpdateRestrictionsResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	if rec.Status == types.AssetStatus_ASSET_STATUS_SETTLED {
		return nil, types.ErrInvalidStatus.Wrap("cannot update restrictions on a settled asset")
	}
	apply := func(list []string, add bool, fnAdd, fnDel func(context.Context, string, sdk.AccAddress) error) error {
		for _, a := range list {
			addr, err := sdk.AccAddressFromBech32(a)
			if err != nil {
				return err
			}
			if add {
				if err := fnAdd(ctx, msg.AssetId, addr); err != nil {
					return err
				}
			} else if err := fnDel(ctx, msg.AssetId, addr); err != nil {
				return err
			}
		}
		return nil
	}
	if err := apply(msg.AddAllow, true, ms.AddAllowlist, ms.DelAllowlist); err != nil {
		return nil, err
	}
	if err := apply(msg.DelAllow, false, ms.AddAllowlist, ms.DelAllowlist); err != nil {
		return nil, err
	}
	if err := apply(msg.AddDeny, true, ms.AddDenylist, ms.DelDenylist); err != nil {
		return nil, err
	}
	if err := apply(msg.DelDeny, false, ms.AddDenylist, ms.DelDenylist); err != nil {
		return nil, err
	}
	rec.AllowAll = msg.AllowAll
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRestrictionUpdated,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyAllowAll, strconv.FormatBool(rec.AllowAll)),
	))
	return &types.MsgUpdateRestrictionsResponse{}, nil
}

func (ms msgServer) SettleRWA(ctx context.Context, msg *types.MsgSettleRWA) (*types.MsgSettleRWAResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, err := ms.requireIssuer(ctx, msg.AssetId, msg.Issuer)
	if err != nil {
		return nil, err
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("settle requires ACTIVE, got %s", rec.Status)
	}
	issuer, err := sdk.AccAddressFromBech32(msg.Issuer)
	if err != nil {
		return nil, err
	}
	notionalMinted, _ := math.NewIntFromString(rec.NotionalMinted)

	params, err := ms.GetParams(ctx)
	if err != nil {
		return nil, err
	}
	rate, err := params.SettleFeeRateDec()
	if err != nil {
		return nil, err
	}
	fee := ms.ComputeFee(notionalMinted, rate)
	if err := ms.CollectFee(ctx, issuer, fee); err != nil {
		return nil, err
	}

	burnCoins := sdk.NewCoins(sdk.NewCoin(rec.Denom, notionalMinted))
	if notionalMinted.IsPositive() {
		if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, issuer, types.ModuleName, burnCoins); err != nil {
			return nil, err
		}
		if err := ms.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
			return nil, err
		}
	}

	bond, _ := math.NewIntFromString(rec.Bond)
	if bond.IsPositive() {
		if err := ms.ReleaseBond(ctx, issuer, bond); err != nil {
			return nil, err
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.AssetStatus_ASSET_STATUS_SETTLED
	rec.SettledAt = sdkCtx.BlockTime()
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRWASettled,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeySettleFee, fee.String()),
		sdk.NewAttribute(types.AttributeKeyBondReturned, bond.String()),
	))
	return &types.MsgSettleRWAResponse{}, nil
}

func (ms msgServer) SlashBond(ctx context.Context, msg *types.MsgSlashBond) (*types.MsgSlashBondResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrUnauthorized, "expected gov authority %s, got %s", ms.GetAuthority(), msg.Authority)
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	rec, found := ms.GetAsset(ctx, msg.AssetId)
	if !found {
		return nil, types.ErrAssetNotFound.Wrap(msg.AssetId)
	}
	if rec.Status != types.AssetStatus_ASSET_STATUS_ACTIVE {
		return nil, types.ErrInvalidStatus.Wrapf("slash requires ACTIVE, got %s", rec.Status)
	}
	bond, _ := math.NewIntFromString(rec.Bond)
	if bond.IsPositive() {
		if err := ms.SlashBondToCommunityPool(ctx, bond); err != nil {
			return nil, err
		}
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.AssetStatus_ASSET_STATUS_SETTLED
	rec.SettledAt = sdkCtx.BlockTime()
	rec.Bond = math.ZeroInt().String()
	if err := ms.SetAsset(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeBondSlashed,
		sdk.NewAttribute(types.AttributeKeyAssetID, rec.AssetId),
		sdk.NewAttribute(types.AttributeKeyAmount, bond.String()),
		sdk.NewAttribute(types.AttributeKeyReason, msg.Reason),
	))
	return &types.MsgSlashBondResponse{}, nil
}

func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "expected %s, got %s", ms.GetAuthority(), msg.Authority)
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(types.EventTypeParamsUpdated))
	return &types.MsgUpdateParamsResponse{}, nil
}
