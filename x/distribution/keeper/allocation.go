package keeper

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/staking/exported"
)

//fundAddress for 70% token reward
const fundAddress = "bhp1su5cjc5lgfwrxtqdz8p0rg6thpxuau2nh8dfx7"

var (
	fundRewardPercent     = sdk.NewDecWithPrec(70, 2) //70% for fund address
	stakingRewardPercent  = sdk.NewDecWithPrec(27, 2) //27% for staking reward
	proposerRewardPercent = sdk.NewDecWithPrec(3, 2)  //3% for proposer reward
)

// AllocateTokens handles distribution of the collected fees
func (k Keeper) AllocateTokens(
	ctx sdk.Context, sumPreviousPrecommitPower, totalPreviousPower int64,
	previousProposer sdk.ConsAddress, previousVotes []abci.VoteInfo,
) {

	logger := k.Logger(ctx)

	// fetch and clear the collected fees for distribution, since this is
	// called in BeginBlock, collected fees will be from the previous block
	// (and distributed to the previous proposer)
	feeCollector := k.supplyKeeper.GetModuleAccount(ctx, k.feeCollectorName)
	feesCollectedInt := feeCollector.GetCoins()
	feesCollected := sdk.NewDecCoins(feesCollectedInt)

	// temporary workaround to keep CanWithdrawInvariant happy
	// general discussions here: https://github.com/cosmos/cosmos-sdk/issues/2906#issuecomment-441867634
	feePool := k.GetFeePool(ctx)
	if totalPreviousPower == 0 {
		feePool.CommunityPool = feePool.CommunityPool.Add(feesCollected)
		k.SetFeePool(ctx, feePool)
		return
	}

	//70% for fund address
	fundAcc, err := sdk.AccAddressFromBech32(fundAddress)
	if err != nil {
		panic(err)
	}
	fundReward := feesCollected.MulDec(fundRewardPercent)
	fundRewardInt, _ := fundReward.TruncateDecimal()
	err = k.supplyKeeper.SendCoinsFromModuleToAccount(ctx, k.feeCollectorName, fundAcc, fundRewardInt)
	if err != nil {
		panic(err)
	}
	//3% for proposer reward
	proposerReward := feesCollected.MulDec(proposerRewardPercent)
	proposerRewardInt, _ := proposerReward.TruncateDecimal()
	err = k.supplyKeeper.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, types.ModuleName, proposerRewardInt)
	if err != nil {
		panic(err)
	}
	//27% for staking reward. calculate remains as staking reward.
	//It is not absolute 27% because of truncateDecimal above.
	stakingRewardInt := feesCollectedInt.Sub(fundRewardInt).Sub(proposerRewardInt)
	stakingReward := sdk.NewDecCoins(stakingRewardInt)
	// transfer collected fees to the distribution module account
	err = k.supplyKeeper.SendCoinsFromModuleToModule(ctx, k.feeCollectorName, types.ModuleName, stakingRewardInt)
	if err != nil {
		panic(err)
	}

	// write 70% 3% 27% reward amount events to chain
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeAllReward,
			sdk.NewAttribute(types.AttributeKeyFundReward, fundRewardInt.String()),
			sdk.NewAttribute(types.AttributeKeyProposerReward, proposerRewardInt.String()),
			sdk.NewAttribute(types.AttributeKeyStakingReward, stakingRewardInt.String()),
		),
	)

	// pay previous proposer
	proposerValidator := k.stakingKeeper.ValidatorByConsAddr(ctx, previousProposer)

	if proposerValidator != nil {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeProposerReward,
				sdk.NewAttribute(sdk.AttributeKeyAmount, proposerReward.String()),
				sdk.NewAttribute(types.AttributeKeyValidator, proposerValidator.GetOperator().String()),
			),
		)

		k.AllocateTokensToValidator(ctx, proposerValidator, proposerReward)
	} else {
		// previous proposer can be unknown if say, the unbonding period is 1 block, so
		// e.g. a validator undelegates at block X, it's removed entirely by
		// block X+1's endblock, then X+2 we need to refer to the previous
		// proposer for X+1, but we've forgotten about them.
		logger.Error(fmt.Sprintf(
			"WARNING: Attempt to allocate proposer rewards to unknown proposer %s. "+
				"This should happen only if the proposer unbonded completely within a single block, "+
				"which generally should not happen except in exceptional circumstances (or fuzz testing). "+
				"We recommend you investigate immediately.",
			previousProposer.String()))
	}

	// calculate fraction allocated to validators
	remaining := stakingReward
	// allocate tokens proportionally to voting power
	// TODO consider parallelizing later, ref https://github.com/cosmos/cosmos-sdk/pull/3099#discussion_r246276376
	for _, vote := range previousVotes {
		validator := k.stakingKeeper.ValidatorByConsAddr(ctx, vote.Validator.Address)

		// TODO consider microslashing for missing votes.
		// ref https://github.com/cosmos/cosmos-sdk/issues/2525#issuecomment-430838701
		powerFraction := sdk.NewDec(vote.Validator.Power).QuoTruncate(sdk.NewDec(totalPreviousPower))
		reward := stakingReward.MulDecTruncate(powerFraction)
		k.AllocateTokensToValidator(ctx, validator, reward)
		remaining = remaining.Sub(reward)
	}

	// allocate community funding
	feePool.CommunityPool = feePool.CommunityPool.Add(remaining)
	k.SetFeePool(ctx, feePool)
}

// AllocateTokensToValidator allocate tokens to a particular validator, splitting according to commission
func (k Keeper) AllocateTokensToValidator(ctx sdk.Context, val exported.ValidatorI, tokens sdk.DecCoins) {
	// split tokens between validator and delegators according to commission
	commission := tokens.MulDec(val.GetCommission())
	shared := tokens.Sub(commission)

	// update current commission
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeCommission,
			sdk.NewAttribute(sdk.AttributeKeyAmount, commission.String()),
			sdk.NewAttribute(types.AttributeKeyValidator, val.GetOperator().String()),
		),
	)
	currentCommission := k.GetValidatorAccumulatedCommission(ctx, val.GetOperator())
	currentCommission = currentCommission.Add(commission)
	k.SetValidatorAccumulatedCommission(ctx, val.GetOperator(), currentCommission)

	// update current rewards
	currentRewards := k.GetValidatorCurrentRewards(ctx, val.GetOperator())
	currentRewards.Rewards = currentRewards.Rewards.Add(shared)
	k.SetValidatorCurrentRewards(ctx, val.GetOperator(), currentRewards)

	// update outstanding rewards
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeRewards,
			sdk.NewAttribute(sdk.AttributeKeyAmount, tokens.String()),
			sdk.NewAttribute(types.AttributeKeyValidator, val.GetOperator().String()),
		),
	)
	outstanding := k.GetValidatorOutstandingRewards(ctx, val.GetOperator())
	outstanding = outstanding.Add(tokens)
	k.SetValidatorOutstandingRewards(ctx, val.GetOperator(), outstanding)
}
