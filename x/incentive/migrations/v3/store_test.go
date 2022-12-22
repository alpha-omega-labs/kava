package v3_test

import (
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/kava-labs/kava/app"
	"github.com/kava-labs/kava/x/incentive/testutil"
	"github.com/kava-labs/kava/x/incentive/types"
	"github.com/stretchr/testify/suite"

	v3 "github.com/kava-labs/kava/x/incentive/migrations/v3"
)

type StoreMigrateTestSuite struct {
	testutil.IntegrationTester

	Addrs []sdk.AccAddress

	keeper   testutil.TestKeeper
	storeKey sdk.StoreKey
	cdc      codec.Codec
}

func TestStoreMigrateTestSuite(t *testing.T) {
	suite.Run(t, new(StoreMigrateTestSuite))
}

func (suite *StoreMigrateTestSuite) SetupTest() {
	suite.IntegrationTester.SetupTest()

	suite.keeper = testutil.TestKeeper{
		Keeper: suite.App.GetIncentiveKeeper(),
	}

	_, suite.Addrs = app.GeneratePrivKeyAddressPairs(5)
	suite.cdc = suite.App.AppCodec()
	suite.storeKey = suite.App.GetKeys()[types.StoreKey]

	suite.StartChain()
}

func (suite *StoreMigrateTestSuite) TestMigrateEarnClaims() {
	store := suite.Ctx.KVStore(suite.storeKey)

	// Create v2 earn claims
	claim1 := types.NewEarnClaim(
		suite.Addrs[0],
		sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
		types.MultiRewardIndexes{
			types.NewMultiRewardIndex("bnb-a", types.RewardIndexes{
				types.NewRewardIndex("bnb", sdk.NewDec(1)),
			}),
		},
	)

	claim2 := types.NewEarnClaim(
		suite.Addrs[1],
		sdk.NewCoins(sdk.NewCoin("usdx", sdk.NewInt(100))),
		types.MultiRewardIndexes{
			types.NewMultiRewardIndex("ukava", types.RewardIndexes{
				types.NewRewardIndex("ukava", sdk.NewDec(1)),
			}),
		},
	)

	suite.keeper.SetEarnClaim(suite.Ctx, claim1)
	suite.keeper.SetEarnClaim(suite.Ctx, claim2)

	// Run earn claim migrations
	err := v3.MigrateEarnClaims(store, suite.cdc)
	suite.Require().NoError(err)

	// Check that the claim was migrated correctly
	newClaim1, found := suite.keeper.Store.GetClaim(suite.Ctx, types.CLAIM_TYPE_EARN, claim1.Owner)
	suite.Require().True(found)
	suite.Require().Equal(claim1.Owner, newClaim1.Owner)

	newClaim2, found := suite.keeper.Store.GetClaim(suite.Ctx, types.CLAIM_TYPE_EARN, claim2.Owner)
	suite.Require().True(found)
	suite.Require().Equal(claim2.Owner, newClaim2.Owner)

	// Ensure removed from old store
	_, found = suite.keeper.GetEarnClaim(suite.Ctx, claim1.Owner)
	suite.Require().False(found)

	_, found = suite.keeper.GetEarnClaim(suite.Ctx, claim2.Owner)
	suite.Require().False(found)
}

func (suite *StoreMigrateTestSuite) TestMigrateHardClaims() {
	store := suite.Ctx.KVStore(suite.storeKey)

	tests := []struct {
		name                   string
		giveV2Claim            types.HardLiquidityProviderClaim
		wantV3ClaimSupplyFound bool
		wantV3ClaimSupply      types.Claim
		wantV3ClaimBorrowFound bool
		wantV3ClaimBorrow      types.Claim
	}{
		{
			name: "supply and borrow",
			giveV2Claim: types.NewHardLiquidityProviderClaim(
				suite.Addrs[0],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-a", types.RewardIndexes{
						types.NewRewardIndex("ukava", sdk.NewDec(1)),
					}),
				},
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-b", types.RewardIndexes{
						types.NewRewardIndex("hard", sdk.NewDec(2)),
					}),
				},
			),
			wantV3ClaimSupplyFound: true,
			wantV3ClaimSupply: types.NewClaim(
				types.CLAIM_TYPE_HARD_SUPPLY,
				suite.Addrs[0],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-a", types.RewardIndexes{
						types.NewRewardIndex("ukava", sdk.NewDec(1)),
					}),
				},
			),
			wantV3ClaimBorrowFound: true,
			wantV3ClaimBorrow: types.NewClaim(
				types.CLAIM_TYPE_HARD_BORROW,
				suite.Addrs[0],
				// Reward coins only in one new claim
				sdk.Coins(nil),
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-b", types.RewardIndexes{
						types.NewRewardIndex("hard", sdk.NewDec(2)),
					}),
				},
			),
		},
		{
			name: "supply only",
			giveV2Claim: types.NewHardLiquidityProviderClaim(
				suite.Addrs[1],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-a", types.RewardIndexes{
						types.NewRewardIndex("ukava", sdk.NewDec(1)),
					}),
				},
				types.MultiRewardIndexes{},
			),
			wantV3ClaimSupplyFound: true,
			wantV3ClaimSupply: types.NewClaim(
				types.CLAIM_TYPE_HARD_SUPPLY,
				suite.Addrs[1],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-a", types.RewardIndexes{
						types.NewRewardIndex("ukava", sdk.NewDec(1)),
					}),
				},
			),
			wantV3ClaimBorrowFound: false,
		},
		{
			name: "borrow only",
			giveV2Claim: types.NewHardLiquidityProviderClaim(
				suite.Addrs[2],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{},
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-b", types.RewardIndexes{
						types.NewRewardIndex("hard", sdk.NewDec(2)),
					}),
				},
			),
			wantV3ClaimSupplyFound: false,
			wantV3ClaimBorrowFound: true,
			wantV3ClaimBorrow: types.NewClaim(
				types.CLAIM_TYPE_HARD_BORROW,
				suite.Addrs[2],
				// Reward coins exists still in the one new claim
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{
					types.NewMultiRewardIndex("bnb-b", types.RewardIndexes{
						types.NewRewardIndex("hard", sdk.NewDec(2)),
					}),
				},
			),
		},
		{
			name: "no reward indexes",
			giveV2Claim: types.NewHardLiquidityProviderClaim(
				suite.Addrs[3],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes{},
				types.MultiRewardIndexes{},
			),
			wantV3ClaimSupplyFound: true,
			wantV3ClaimSupply: types.NewClaim(
				types.CLAIM_TYPE_HARD_SUPPLY,
				suite.Addrs[3],
				sdk.NewCoins(sdk.NewCoin("bnb", sdk.NewInt(100))),
				types.MultiRewardIndexes(nil),
			),
			wantV3ClaimBorrowFound: false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.keeper.SetHardLiquidityProviderClaim(suite.Ctx, tt.giveV2Claim)

			err := v3.MigrateHardClaims(store, suite.cdc)
			suite.Require().NoError(err)

			// Check supply claim
			newClaimSupply, found := suite.keeper.Store.GetClaim(suite.Ctx, types.CLAIM_TYPE_HARD_SUPPLY, tt.giveV2Claim.Owner)
			if tt.wantV3ClaimSupplyFound {
				suite.Require().True(found)
				suite.Require().Equal(tt.wantV3ClaimSupply, newClaimSupply)
			} else {
				suite.Require().Falsef(found, "expected supply claim to not be found, but it was found: %s", newClaimSupply)
			}

			// Check borrow claim
			newClaimBorrow, found := suite.keeper.Store.GetClaim(suite.Ctx, types.CLAIM_TYPE_HARD_BORROW, tt.giveV2Claim.Owner)
			if tt.wantV3ClaimBorrowFound {
				suite.Require().True(found)
				suite.Require().Equal(tt.wantV3ClaimBorrow, newClaimBorrow)
			} else {
				suite.Require().Falsef(found, "expected borrow claim to not be found, but it was found: %s", newClaimBorrow)
			}

			totalNewReward := newClaimBorrow.Reward.Add(newClaimSupply.Reward...)
			suite.Require().Equal(tt.giveV2Claim.Reward, totalNewReward, "total new reward coins should equal old reward coins")

			// Check old claim is deleted
			_, found = suite.keeper.GetHardLiquidityProviderClaim(suite.Ctx, tt.giveV2Claim.Owner)
			suite.Require().False(found)
		})
	}
}

func (suite *StoreMigrateTestSuite) TestMigrateAccrualTimes() {
	store := suite.Ctx.KVStore(suite.storeKey)
	denom1 := "ukava"
	denom2 := "usdc"

	for i, claimType := range v3.MigrateClaimTypes {
		suite.Run(claimType.String(), func() {
			// Create v2 accrual times, make each claimType have a different time
			accrualTime1 := time.Now().Add(time.Duration(i) * time.Hour)
			accrualTime2 := accrualTime1.Add(time.Hour * 24)

			switch claimType {
			case types.CLAIM_TYPE_EARN:
				suite.keeper.SetEarnRewardAccrualTime(suite.Ctx, denom1, accrualTime1)
				suite.keeper.SetEarnRewardAccrualTime(suite.Ctx, denom2, accrualTime2)
			case types.CLAIM_TYPE_HARD_SUPPLY:
				suite.keeper.SetPreviousHardSupplyRewardAccrualTime(suite.Ctx, denom1, accrualTime1)
				suite.keeper.SetPreviousHardSupplyRewardAccrualTime(suite.Ctx, denom2, accrualTime2)
			case types.CLAIM_TYPE_HARD_BORROW:
				suite.keeper.SetPreviousHardBorrowRewardAccrualTime(suite.Ctx, denom1, accrualTime1)
				suite.keeper.SetPreviousHardBorrowRewardAccrualTime(suite.Ctx, denom2, accrualTime2)
			}

			// Run accrual time migrations
			err := v3.MigrateAccrualTimes(store, suite.cdc, claimType)
			suite.Require().NoError(err)

			// Check that the accrual time was migrated correctly
			newAccrualTime1, found := suite.keeper.Store.GetRewardAccrualTime(suite.Ctx, claimType, denom1)
			suite.Require().True(found)
			suite.Require().Equal(accrualTime1.Unix(), newAccrualTime1.Unix())

			newAccrualTime2, found := suite.keeper.Store.GetRewardAccrualTime(suite.Ctx, claimType, denom2)
			suite.Require().True(found)
			suite.Require().Equal(accrualTime2.Unix(), newAccrualTime2.Unix())

			var found1, found2 bool

			// Ensure removed from old store
			switch claimType {
			case types.CLAIM_TYPE_EARN:
				_, found1 = suite.keeper.GetEarnRewardAccrualTime(suite.Ctx, denom1)
				_, found2 = suite.keeper.GetEarnRewardAccrualTime(suite.Ctx, denom2)
			case types.CLAIM_TYPE_HARD_SUPPLY:
				_, found1 = suite.keeper.GetPreviousHardSupplyRewardAccrualTime(suite.Ctx, denom1)
				_, found2 = suite.keeper.GetPreviousHardSupplyRewardAccrualTime(suite.Ctx, denom2)
			case types.CLAIM_TYPE_HARD_BORROW:
				_, found1 = suite.keeper.GetPreviousHardSupplyRewardAccrualTime(suite.Ctx, denom1)
				_, found2 = suite.keeper.GetPreviousHardSupplyRewardAccrualTime(suite.Ctx, denom2)
			}

			suite.Require().Falsef(found1, "expected old accrual time 1 to be deleted, but it was found for claim type %s", claimType)
			suite.Require().Falsef(found2, "expected old accrual time 2 to be deleted, but it was found for claim type %s", claimType)
		})
	}
}

func (suite *StoreMigrateTestSuite) TestMigrateRewardIndexes() {
	store := suite.Ctx.KVStore(suite.storeKey)
	denom1 := "ukava"
	denom2 := "usdc"

	rewardIndexes1 := types.RewardIndexes{
		types.NewRewardIndex("ukava", sdk.NewDec(1)),
		types.NewRewardIndex("hard", sdk.NewDec(2)),
	}
	rewardIndexes2 := types.RewardIndexes{
		types.NewRewardIndex("ukava", sdk.NewDec(4)),
		types.NewRewardIndex("swp", sdk.NewDec(10)),
	}

	for _, claimType := range v3.MigrateClaimTypes {
		suite.Run(claimType.String(), func() {
			switch claimType {
			case types.CLAIM_TYPE_EARN:
				suite.keeper.SetEarnRewardIndexes(suite.Ctx, denom1, rewardIndexes1)
				suite.keeper.SetEarnRewardIndexes(suite.Ctx, denom2, rewardIndexes2)
			case types.CLAIM_TYPE_HARD_SUPPLY:
				suite.keeper.SetHardSupplyRewardIndexes(suite.Ctx, denom1, rewardIndexes1)
				suite.keeper.SetHardSupplyRewardIndexes(suite.Ctx, denom2, rewardIndexes2)
			case types.CLAIM_TYPE_HARD_BORROW:
				suite.keeper.SetHardBorrowRewardIndexes(suite.Ctx, denom1, rewardIndexes1)
				suite.keeper.SetHardBorrowRewardIndexes(suite.Ctx, denom2, rewardIndexes2)
			}

			err := v3.MigrateRewardIndexes(store, suite.cdc, claimType)
			suite.Require().NoError(err)

			newRewardIndexes1, found := suite.keeper.Store.GetRewardIndexesOfClaimType(suite.Ctx, claimType, denom1)
			suite.Require().True(found)
			suite.Require().Equal(rewardIndexes1, newRewardIndexes1)

			newRewardIndexes2, found := suite.keeper.Store.GetRewardIndexesOfClaimType(suite.Ctx, claimType, denom2)
			suite.Require().True(found)
			suite.Require().Equal(rewardIndexes2, newRewardIndexes2)

			var found1, found2 bool

			// Ensure removed from old store
			switch claimType {
			case types.CLAIM_TYPE_EARN:
				_, found1 = suite.keeper.GetEarnRewardIndexes(suite.Ctx, denom1)
				_, found2 = suite.keeper.GetEarnRewardIndexes(suite.Ctx, denom2)
			case types.CLAIM_TYPE_HARD_SUPPLY:
				_, found1 = suite.keeper.GetHardSupplyRewardIndexes(suite.Ctx, denom1)
				_, found2 = suite.keeper.GetHardSupplyRewardIndexes(suite.Ctx, denom2)
			case types.CLAIM_TYPE_HARD_BORROW:
				_, found1 = suite.keeper.GetHardBorrowRewardIndexes(suite.Ctx, denom1)
				_, found2 = suite.keeper.GetHardBorrowRewardIndexes(suite.Ctx, denom2)
			}

			suite.Require().Falsef(found1, "expected old reward indexes 1 to be deleted, but it was found for claim type %s", claimType)
			suite.Require().Falsef(found2, "expected old reward indexes 2 to be deleted, but it was found for claim type %s", claimType)
		})
	}
}
