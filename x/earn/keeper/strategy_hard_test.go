package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/kava-labs/kava/x/earn/testutil"
	"github.com/kava-labs/kava/x/earn/types"
	"github.com/kava-labs/kava/x/hard"

	hardtypes "github.com/kava-labs/kava/x/hard/types"

	"github.com/stretchr/testify/suite"
)

type strategyHardTestSuite struct {
	testutil.Suite
}

func (suite *strategyHardTestSuite) SetupTest() {
	suite.Suite.SetupTest()
	suite.Keeper.SetParams(suite.Ctx, types.DefaultParams())

	usdxMoneyMarket := hardtypes.NewMoneyMarket(
		"usdx",
		hardtypes.NewBorrowLimit(
			true,
			sdk.MustNewDecFromStr("20000000"),
			sdk.MustNewDecFromStr("1"),
		),
		"usdx:usd",
		sdk.NewInt(1000000),
		hardtypes.NewInterestRateModel(
			sdk.MustNewDecFromStr("0.05"),
			sdk.MustNewDecFromStr("2"),
			sdk.MustNewDecFromStr("0.8"),
			sdk.MustNewDecFromStr("10"),
		),
		sdk.MustNewDecFromStr("0.05"),
		sdk.ZeroDec(),
	)

	suite.HardKeeper.SetParams(
		suite.Ctx,
		hardtypes.NewParams(
			hardtypes.MoneyMarkets{
				usdxMoneyMarket,
			},
			sdk.NewDec(10),
		))

	suite.HardKeeper.SetMoneyMarket(suite.Ctx, "usdx", usdxMoneyMarket)
	hard.BeginBlocker(suite.Ctx, suite.HardKeeper)
}

func TestStrategyLendTestSuite(t *testing.T) {
	suite.Run(t, new(strategyHardTestSuite))
}

func (suite *strategyHardTestSuite) TestDeposit_SingleAcc() {
	vaultDenom := "usdx"
	startBalance := sdk.NewInt64Coin(vaultDenom, 1000)
	depositAmount := sdk.NewInt64Coin(vaultDenom, 100)

	suite.CreateVault(vaultDenom, types.STRATEGY_TYPE_HARD)

	acc := suite.CreateAccount(sdk.NewCoins(startBalance), 0)

	err := suite.Keeper.Deposit(suite.Ctx, acc.GetAddress(), depositAmount)
	suite.Require().NoError(err)

	suite.HardDepositAmountEqual(sdk.NewCoins(depositAmount))

	// Query vault total
	totalValue, err := suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenom)
	suite.Require().NoError(err)

	suite.Equal(depositAmount, totalValue)
}

func (suite *strategyHardTestSuite) TestDeposit_SingleAcc_MultipleDeposits() {
	vaultDenom := "usdx"
	startBalance := sdk.NewInt64Coin(vaultDenom, 1000)
	depositAmount := sdk.NewInt64Coin(vaultDenom, 100)

	suite.CreateVault(vaultDenom, types.STRATEGY_TYPE_HARD)

	acc := suite.CreateAccount(sdk.NewCoins(startBalance), 0)

	err := suite.Keeper.Deposit(suite.Ctx, acc.GetAddress(), depositAmount)
	suite.Require().NoError(err)

	// Second deposit
	err = suite.Keeper.Deposit(suite.Ctx, acc.GetAddress(), depositAmount)
	suite.Require().NoError(err)

	suite.HardDepositAmountEqual(sdk.NewCoins(depositAmount.Add(depositAmount)))

	// Query vault total
	totalValue, err := suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenom)
	suite.Require().NoError(err)

	suite.Equal(depositAmount.Add(depositAmount), totalValue)
}

func (suite *strategyHardTestSuite) TestDeposit_MultipleAcc_MultipleDeposits() {
	vaultDenom := "usdx"
	startBalance := sdk.NewInt64Coin(vaultDenom, 1000)
	depositAmount := sdk.NewInt64Coin(vaultDenom, 100)

	expectedTotalValue := sdk.NewCoin(vaultDenom, depositAmount.Amount.MulRaw(4))

	suite.CreateVault(vaultDenom, types.STRATEGY_TYPE_HARD)

	acc1 := suite.CreateAccount(sdk.NewCoins(startBalance), 0)
	acc2 := suite.CreateAccount(sdk.NewCoins(startBalance), 0)

	// 2 deposits each account
	for i := 0; i < 2; i++ {
		// Deposit from acc1
		err := suite.Keeper.Deposit(suite.Ctx, acc1.GetAddress(), depositAmount)
		suite.Require().NoError(err)

		// Deposit from acc2
		err = suite.Keeper.Deposit(suite.Ctx, acc2.GetAddress(), depositAmount)
		suite.Require().NoError(err)
	}

	suite.HardDepositAmountEqual(sdk.NewCoins(expectedTotalValue))

	// Query vault total
	totalValue, err := suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenom)
	suite.Require().NoError(err)

	suite.Equal(expectedTotalValue, totalValue)
}

func (suite *strategyHardTestSuite) TestGetVaultTotalValue_Empty() {
	vaultDenom := "usdx"

	suite.CreateVault(vaultDenom, types.STRATEGY_TYPE_HARD)

	// Query vault total
	totalValue, err := suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenom)
	suite.Require().NoError(err)

	suite.Equal(sdk.NewCoin(vaultDenom, sdk.ZeroInt()), totalValue)
}

func (suite *strategyHardTestSuite) TestGetVaultTotalValue_NoDenomDeposit() {
	// 2 Vaults usdx, busd
	// 1st vault has deposits
	// 2nd vault has no deposits
	vaultDenom := "usdx"
	vaultDenomBusd := "busd"

	suite.CreateVault(vaultDenom, types.STRATEGY_TYPE_HARD)
	suite.CreateVault(vaultDenomBusd, types.STRATEGY_TYPE_HARD)

	startBalance := sdk.NewInt64Coin(vaultDenom, 1000)
	depositAmount := sdk.NewInt64Coin(vaultDenom, 100)

	acc := suite.CreateAccount(sdk.NewCoins(startBalance), 0)

	// Deposit vault1
	err := suite.Keeper.Deposit(suite.Ctx, acc.GetAddress(), depositAmount)
	suite.Require().NoError(err)

	// Query vault total, hard deposit exists for account, but amount in busd does not
	// Vault2 does not have any value, only returns amount for the correct denom
	// if a hard deposit already exists
	totalValueBusd, err := suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenomBusd)
	suite.Require().NoError(err)

	suite.Equal(sdk.NewCoin(vaultDenomBusd, sdk.ZeroInt()), totalValueBusd)
}

// ----------------------------------------------------------------------------
// Withdraw

func (suite *strategyHardTestSuite) TestWithdraw() {
	vaultDenom := "usdx"
	startBalance := sdk.NewInt64Coin(vaultDenom, 1000)
	depositAmount := sdk.NewInt64Coin(vaultDenom, 100)

	suite.CreateVault(vaultDenom, types.STRATEGY_TYPE_HARD)

	acc := suite.CreateAccount(sdk.NewCoins(startBalance), 0)
	err := suite.Keeper.Deposit(suite.Ctx, acc.GetAddress(), depositAmount)
	suite.Require().NoError(err)

	suite.HardDepositAmountEqual(sdk.NewCoins(depositAmount))

	// Query vault total
	totalValue, err := suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenom)
	suite.Require().NoError(err)
	suite.Equal(depositAmount, totalValue)

	// Withdraw
	err = suite.Keeper.Withdraw(suite.Ctx, acc.GetAddress(), depositAmount)
	suite.Require().NoError(err)

	suite.HardDepositAmountEqual(sdk.NewCoins())

	totalValue, err = suite.Keeper.GetVaultTotalValue(suite.Ctx, vaultDenom)
	suite.Require().NoError(err)
	suite.Equal(sdk.NewInt64Coin(vaultDenom, 0), totalValue)
}
