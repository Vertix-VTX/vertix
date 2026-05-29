package keeper

import (
	"context"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/vertix-network/vertix/x/oracle/types"
)

// EndBlocker aggregates oracle feeds at vote-window boundaries.
func (k Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := sdkCtx.BlockHeight()

	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	if height%params.VoteWindow != 0 {
		return nil
	}

	vals, err := k.stakingKeeper.GetBondedValidatorsByPower(ctx)
	if err != nil {
		return err
	}
	valByOper := make(map[string]stakingtypes.Validator, len(vals))
	for _, v := range vals {
		valByOper[v.OperatorAddress] = v
	}

	totalPower, err := k.stakingKeeper.GetLastTotalPower(ctx)
	if err != nil {
		return err
	}

	quorumFraction, err := math.LegacyNewDecFromStr(params.QuorumFraction)
	if err != nil {
		return err
	}
	outlierThreshold, err := math.LegacyNewDecFromStr(params.OutlierThreshold)
	if err != nil {
		return err
	}
	outlierSlashRate, err := math.LegacyNewDecFromStr(params.OutlierSlashRate)
	if err != nil {
		return err
	}
	missThreshold, err := math.LegacyNewDecFromStr(params.MissThreshold)
	if err != nil {
		return err
	}
	missSlashRate, err := math.LegacyNewDecFromStr(params.MissSlashRate)
	if err != nil {
		return err
	}

	blockTime := sdkCtx.BlockTime()
	twapCutoff := blockTime.Add(-24 * time.Hour)
	quorumLive := make([]string, 0)

	for _, pair := range params.AcceptList {
		var weighted []WeightedPrice
		rawPrices := make([]math.LegacyDec, 0)
		type validFeed struct {
			valoper string
			price   math.LegacyDec
		}
		validFeeds := make([]validFeed, 0)
		submittedPower := math.ZeroInt()

		err := k.IterateAllFeeds(ctx, func(val sdk.ValAddress, feedPair, priceStr string) bool {
			if feedPair != pair {
				return true
			}
			price, err := math.LegacyNewDecFromStr(priceStr)
			if err != nil || !price.IsPositive() {
				return true
			}
			v, ok := valByOper[val.String()]
			if !ok || v.Status != stakingtypes.Bonded {
				return true
			}
			pw, err := k.stakingKeeper.GetLastValidatorPower(ctx, val)
			if err != nil || pw == 0 {
				return true
			}
			weighted = append(weighted, WeightedPrice{Price: price, Weight: pw, Valoper: val.String()})
			rawPrices = append(rawPrices, price)
			validFeeds = append(validFeeds, validFeed{valoper: val.String(), price: price})
			submittedPower = submittedPower.Add(math.NewInt(pw))
			return true
		})
		if err != nil {
			return err
		}
		if len(weighted) == 0 {
			continue
		}
		if !QuorumMet(submittedPower, totalPower, quorumFraction) {
			continue
		}

		quorumLive = append(quorumLive, pair)

		median := WeightedMedian(weighted)
		refMedian := UnweightedMedian(rawPrices)

		if err := k.SetAggregatedPrice(ctx, types.AggregatedPrice{
			Pair:        pair,
			Price:       median.String(),
			BlockHeight: height,
			BlockTime:   blockTime,
		}); err != nil {
			return err
		}
		if err := k.AppendTWAPEntry(ctx, pair, median, blockTime); err != nil {
			return err
		}
		if err := k.PruneTWAPOlderThan(ctx, pair, twapCutoff); err != nil {
			return err
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypePriceAggregated,
			sdk.NewAttribute(types.AttributeKeyPair, pair),
			sdk.NewAttribute(types.AttributeKeyPrice, median.String()),
		))

		if refMedian.IsPositive() {
			for _, f := range validFeeds {
				diff := f.price.Sub(refMedian).Abs()
				if diff.Quo(refMedian).GT(outlierThreshold) {
					k.slashValidator(ctx, valByOper[f.valoper], outlierSlashRate, "outlier")
				}
			}
		}
	}

	for _, v := range vals {
		valAddr, err := sdk.ValAddressFromBech32(v.OperatorAddress)
		if err != nil {
			continue
		}
		submitted, err := k.GetValidatorSubmittedPairs(ctx, valAddr)
		if err != nil {
			return err
		}
		submittedAllLive := true
		for _, pair := range quorumLive {
			if _, ok := submitted[pair]; !ok {
				submittedAllLive = false
				break
			}
		}

		total, err := k.IncTotalWindows(ctx, valAddr)
		if err != nil {
			return err
		}
		var miss int64
		if !submittedAllLive {
			miss, err = k.IncMissCounter(ctx, valAddr)
			if err != nil {
				return err
			}
		} else {
			miss, err = k.GetMissCounter(ctx, valAddr)
			if err != nil {
				return err
			}
		}

		if total >= params.MissWindowSize {
			missRate := math.LegacyNewDec(miss).Quo(math.LegacyNewDec(total))
			if missRate.GT(missThreshold) {
				k.slashValidator(ctx, v, missSlashRate, "miss")
			}
			if err := k.ResetValidatorWindows(ctx, valAddr); err != nil {
				return err
			}
		}
	}

	return k.DeleteAllFeeds(ctx)
}

func (k Keeper) slashValidator(ctx context.Context, v stakingtypes.Validator, rate math.LegacyDec, reason string) {
	consAddr, err := v.GetConsAddr()
	if err != nil || consAddr == nil {
		return
	}
	valAddr, err := sdk.ValAddressFromBech32(v.OperatorAddress)
	if err != nil {
		return
	}
	power, err := k.stakingKeeper.GetLastValidatorPower(ctx, valAddr)
	if err != nil || power == 0 {
		return
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := k.slashingKeeper.Slash(sdkCtx, consAddr, rate, power, sdkCtx.BlockHeight()); err != nil {
		return
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeSlash,
		sdk.NewAttribute(types.AttributeKeyValidator, v.OperatorAddress),
		sdk.NewAttribute(types.AttributeKeySlashReason, reason),
		sdk.NewAttribute(types.AttributeKeySlashFraction, rate.String()),
	))
}
