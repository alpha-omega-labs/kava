package keeper_test

import (
	"testing"

	"github.com/kava-labs/kava/x/evmutil/keeper"
	"github.com/kava-labs/kava/x/evmutil/testutil"
	"github.com/kava-labs/kava/x/evmutil/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

type invariantTestSuite struct {
	testutil.Suite
	invariants map[string]map[string]sdk.Invariant
}

func TestInvariantTestSuite(t *testing.T) {
	suite.Run(t, new(invariantTestSuite))
}

func (suite *invariantTestSuite) SetupTest() {
	suite.Suite.SetupTest()
	suite.invariants = make(map[string]map[string]sdk.Invariant)
	keeper.RegisterInvariants(suite, suite.EvmBankKeeper, suite.Keeper)
}

func (suite *invariantTestSuite) SetupValidState() {
	for i := 0; i < 4; i++ {
		suite.Keeper.SetAccount(suite.Ctx, *types.NewAccount(
			suite.Addrs[i],
			sdk.NewInt(5e11),
		))
	}
	suite.FundModuleAccountWithKava(
		types.ModuleName,
		sdk.NewCoins(
			sdk.NewCoin("ukava", sdk.NewInt(2)),
		),
	)
}

// RegisterRoutes implements sdk.InvariantRegistry
func (suite *invariantTestSuite) RegisterRoute(moduleName string, route string, invariant sdk.Invariant) {
	_, exists := suite.invariants[moduleName]

	if !exists {
		suite.invariants[moduleName] = make(map[string]sdk.Invariant)
	}

	suite.invariants[moduleName][route] = invariant
}

func (suite *invariantTestSuite) runInvariant(route string, invariant func(bankKeeper keeper.EvmBankKeeper, k keeper.Keeper) sdk.Invariant) (string, bool) {
	ctx := suite.Ctx
	registeredInvariant := suite.invariants[types.ModuleName][route]
	suite.Require().NotNil(registeredInvariant)

	// direct call
	dMessage, dBroken := invariant(suite.EvmBankKeeper, suite.Keeper)(ctx)
	// registered call
	rMessage, rBroken := registeredInvariant(ctx)
	// all call
	aMessage, aBroken := keeper.AllInvariants(suite.EvmBankKeeper, suite.Keeper)(ctx)

	// require matching values for direct call and registered call
	suite.Require().Equal(dMessage, rMessage, "expected registered invariant message to match")
	suite.Require().Equal(dBroken, rBroken, "expected registered invariant broken to match")
	// require matching values for direct call and all invariants call if broken
	suite.Require().Equal(dBroken, aBroken, "expected all invariant broken to match")
	if dBroken {
		suite.Require().Equal(dMessage, aMessage, "expected all invariant message to match")
	}

	return dMessage, dBroken
}

func (suite *invariantTestSuite) TestBalancesInvariant() {

	// default state is valid
	_, broken := suite.runInvariant("balances", keeper.BalancesInvariant)
	suite.Equal(false, broken)

	suite.SetupValidState()
	_, broken = suite.runInvariant("balances", keeper.BalancesInvariant)
	suite.Equal(false, broken)

	// break invariant by decreasing minor balances without decreasing module balance
	suite.Keeper.RemoveBalance(suite.Ctx, suite.Addrs[0], sdk.OneInt())

	message, broken := suite.runInvariant("balances", keeper.BalancesInvariant)
	suite.Equal("evmutil: balances broken invariant\nminor balances do not match module account\n", message)
	suite.Equal(true, broken)
}
