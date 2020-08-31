package mint

// nolint

import (
	"github.com/cosmos/cosmos-sdk/x/mint/internal/keeper"
	"github.com/cosmos/cosmos-sdk/x/mint/internal/types"
)

const (
	ModuleName            = types.ModuleName
)

var (
	// functions aliases
	NewKeeper            = keeper.NewKeeper
)

type (
	Keeper       = keeper.Keeper
)
