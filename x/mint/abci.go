package mint

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/mint/internal/types"
)

var (

	// SatoshiPerBitcoin is the number of satoshi in one bitcoin (1 BTC).
	// SatoshiPerBitcoin = 1e8
	abhpPerBhp = sdk.PowerReduction.BigInt()

	// baseSubsidy is the starting subsidy amount for mined blocks.  This
	// value is halved every SubsidyHalvingInterval blocks.
	// baseSubsidy = 50 * SatoshiPerBitcoin
	baseSubsidy = new(big.Int).Mul(abhpPerBhp, big.NewInt(50))

	// SubsidyReductionInterval is the halving interval
	// SubsidyReductionInterval = 210000
	SubsidyReductionInterval = int64(210000 * 40)
)

// CalcBlockSubsidy returns the subsidy amount a block at the provided height
// should have. This is mainly used for determining how much the coinbase for
// newly generated blocks awards as well as validating the coinbase for blocks
// has the expected value.
//
// The subsidy is halved every SubsidyReductionInterval blocks.  Mathematically
// this is: baseSubsidy / 2^(height/SubsidyReductionInterval)
//
// At the target block generation rate for the main network, this is
// approximately every 4 years.
func CalcBlockSubsidy(height int64) *big.Int {

	// Equivalent to: baseSubsidy / 2^(height/subsidyHalvingInterval)
	// return baseSubsidy >> uint(height/SubsidyReductionInterval)
	rsh := uint(height / SubsidyReductionInterval)
	return new(big.Int).Rsh(baseSubsidy, rsh)
}

// BeginBlocker mints new tokens for the previous block.
func BeginBlocker(ctx sdk.Context, k Keeper) {

	blockSubsidy := CalcBlockSubsidy(ctx.BlockHeight())
	mintedCoin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewIntFromBigInt(blockSubsidy))
	mintedCoins := sdk.NewCoins(mintedCoin)

	err := k.MintCoins(ctx, mintedCoins)
	if err != nil {
		panic(err)
	}

	// send the minted coins to the fee collector account
	err = k.AddCollectedFees(ctx, mintedCoins)
	if err != nil {
		panic(err)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeMint,
			sdk.NewAttribute(sdk.AttributeKeyAmount, mintedCoin.Amount.String()),
		),
	)
}
