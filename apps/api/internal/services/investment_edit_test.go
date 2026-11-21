package services

import (
	"testing"
	"time"

	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/testutil"
)

// cashLegOf returns the non-deleted cash rows linked to an investment transaction.
func cashLegOf(t *testing.T, f *nativeFixture, invTxID string) []models.Transaction {
	t.Helper()
	var rows []models.Transaction
	testutil.AssertNoError(t, f.db.Where("investment_transaction_id = ?", invTxID).Find(&rows).Error)
	return rows
}

func TestRecordTrades_LinkCashLeg(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)

	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	if legs := cashLegOf(t, f, buy.ID); len(legs) != 1 || legs[0].Amount != 10000 {
		t.Fatalf("buy cash legs = %+v", legs)
	}
	if buy.CashAccountID == nil || *buy.CashAccountID != f.cashMYR.ID || buy.CashAmount == nil || *buy.CashAmount != 10000 {
		t.Errorf("buy response cash leg = %v / %v", buy.CashAccountID, buy.CashAmount)
	}
	div, err := f.svc.RecordDividend(f.user.ID, inv.ID, DividendParams{Date: nativeDay, Amount: 500, Fee: 50, DepositAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	if legs := cashLegOf(t, f, div.ID); len(legs) != 1 || legs[0].Amount != 450 {
		t.Fatalf("dividend cash legs = %+v", legs)
	}
}

func TestDeleteInvestmentTransaction_ReversesCashAndRebuilds(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	start := f.balance(t, f.cashMYR)

	_, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	typo, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay.Add(time.Hour), Quantity: 100, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	if got := f.balance(t, f.cashMYR); got != start-110000 {
		t.Fatalf("cash after buys = %d", got)
	}

	testutil.AssertNoError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, inv.ID, typo.ID))

	if got := f.balance(t, f.cashMYR); got != start-10000 {
		t.Errorf("cash after delete = %d, want %d", got, start-10000)
	}
	got := f.reload(t, inv)
	if got.Quantity != 10 || got.CostBasis != 10000 || got.CostBasisBase != 10000 {
		t.Errorf("holding = %+v", got)
	}
	if legs := cashLegOf(t, f, typo.ID); len(legs) != 0 {
		t.Errorf("cash leg not deleted: %+v", legs)
	}
	_, err = f.svc.GetInvestmentTransactions(f.user.ID, inv.ID, pagination.PageRequest{})
	testutil.AssertNoError(t, err)
	testutil.AssertAppError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, inv.ID, typo.ID), "INVESTMENT_TRANSACTION_NOT_FOUND")
}

func TestDeleteInvestmentTransaction_RefusesOversell(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	_, err = f.svc.RecordSell(f.user.ID, inv.ID, SellParams{Date: nativeDay.Add(time.Hour), Quantity: 5, PricePerUnit: 1200})
	testutil.AssertNoError(t, err)
	before := f.balance(t, f.cashMYR)

	testutil.AssertAppError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, inv.ID, buy.ID), "INSUFFICIENT_SHARES")

	if got := f.balance(t, f.cashMYR); got != before {
		t.Errorf("cash changed on a refused delete: %d -> %d", before, got)
	}
	if got := f.reload(t, inv); got.Quantity != 5 {
		t.Errorf("quantity = %v", got.Quantity)
	}
	if legs := cashLegOf(t, f, buy.ID); len(legs) != 1 {
		t.Errorf("cash leg gone after a refused delete")
	}
}

func TestDeleteInvestmentTransaction_OtherUser(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 1, PricePerUnit: 1000})
	testutil.AssertNoError(t, err)
	other := testutil.CreateTestUserWithEmail(t, f.db, "other@example.com")
	testutil.AssertAppError(t, f.svc.DeleteInvestmentTransaction(other.ID, inv.ID, buy.ID), "INVESTMENT_NOT_FOUND")
}

func TestUpdateInvestmentTransaction_BuyRecomputesLaterSell(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	start := f.balance(t, f.cashMYR)

	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 10000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	sell, err := f.svc.RecordSell(f.user.ID, inv.ID, SellParams{Date: nativeDay.Add(time.Hour), Quantity: 5, PricePerUnit: 1200, DepositAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)

	// The buy price was mistyped: 100.00 instead of 10.00.
	updated, err := f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, buy.ID, UpdateTradeParams{
		Date: nativeDay, Quantity: 10, PricePerUnit: 1000, Notes: "fixed", AccountID: f.cashMYR.ID,
	})
	testutil.AssertNoError(t, err)
	if updated.ID != buy.ID || updated.TotalAmount != 10000 || updated.Notes != "fixed" {
		t.Errorf("updated = %+v", updated)
	}
	wantRate(t, updated, "1", 10000)

	if got := f.balance(t, f.cashMYR); got != start-10000+6000 {
		t.Errorf("cash = %d, want %d", got, start-10000+6000)
	}
	got := f.reload(t, inv)
	if got.Quantity != 5 || got.CostBasis != 5000 || got.RealizedGainLoss != 1000 {
		t.Errorf("holding = %+v", got)
	}
	var s models.InvestmentTransaction
	testutil.AssertNoError(t, f.db.First(&s, "id = ?", sell.ID).Error)
	if s.RealizedGainLoss != 1000 || s.RealizedGainLossBase != 1000 {
		t.Errorf("sell realized = %d / %d, want 1000", s.RealizedGainLoss, s.RealizedGainLossBase)
	}
	if legs := cashLegOf(t, f, buy.ID); len(legs) != 1 || legs[0].Amount != 10000 {
		t.Errorf("buy cash legs = %+v", legs)
	}
}

func TestUpdateInvestmentTransaction_RefusesOversell(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000})
	testutil.AssertNoError(t, err)
	_, err = f.svc.RecordSell(f.user.ID, inv.ID, SellParams{Date: nativeDay.Add(time.Hour), Quantity: 8, PricePerUnit: 1000})
	testutil.AssertNoError(t, err)

	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, buy.ID, UpdateTradeParams{Date: nativeDay, Quantity: 5, PricePerUnit: 1000})
	testutil.AssertAppError(t, err, "INSUFFICIENT_SHARES")
	var b models.InvestmentTransaction
	testutil.AssertNoError(t, f.db.First(&b, "id = ?", buy.ID).Error)
	if b.Quantity != 10 {
		t.Errorf("buy changed on a refused edit: %v", b.Quantity)
	}
}

func TestUpdateInvestmentTransaction_SameEconomicsKeepsRateAndLeg(t *testing.T) {
	f := newNativeFixture(t, true)
	inv := f.holding(t, f.usd)
	// USD security paid from the MYR account: the paid amount implies the rate.
	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{
		Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID, FundingAmount: 45000,
	})
	testutil.AssertNoError(t, err)
	wantRate(t, buy, "4.5", 45000)
	legBefore := cashLegOf(t, f, buy.ID)

	// The market rate moves; a notes-only edit must not re-price the trade.
	seedRate(t, f.fx, "USD", "MYR", "5", nativeDay.Add(time.Minute))
	updated, err := f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, buy.ID, UpdateTradeParams{
		Date: nativeDay, Quantity: 10, PricePerUnit: 1000, Notes: "note", AccountID: f.cashMYR.ID, AccountAmount: 45000,
	})
	testutil.AssertNoError(t, err)
	wantRate(t, updated, "4.5", 45000)
	legAfter := cashLegOf(t, f, buy.ID)
	if len(legAfter) != 1 || legAfter[0].ID != legBefore[0].ID {
		t.Errorf("cash leg replaced on an unchanged edit: %+v -> %+v", legBefore, legAfter)
	}

	// Paying a different amount re-derives the rate from it.
	updated, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, buy.ID, UpdateTradeParams{
		Date: nativeDay, Quantity: 10, PricePerUnit: 1000, AccountID: f.cashMYR.ID, AccountAmount: 44000,
	})
	testutil.AssertNoError(t, err)
	wantRate(t, updated, "4.4", 44000)
	if got := f.balance(t, f.cashMYR); got != 10_000_000-44000 {
		t.Errorf("cash = %d", got)
	}
	if got := f.reload(t, inv); got.CostBasisBase != 44000 || got.CostBasis != 10000 {
		t.Errorf("holding = %+v", got)
	}
}

func TestUpdateInvestmentTransaction_MoveAndDropCashLeg(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	other := testutil.CreateTestCashAccountWithBalance(t, f.db, f.user.ID, 0)
	testutil.AssertNoError(t, f.db.Model(other).Update("currency", "MYR").Error)
	start := f.balance(t, f.cashMYR)

	div, err := f.svc.RecordDividend(f.user.ID, inv.ID, DividendParams{Date: nativeDay, Amount: 1000, DepositAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)

	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, div.ID, UpdateTradeParams{
		Date: nativeDay, Amount: 1000, Fee: 100, AccountID: other.ID,
	})
	testutil.AssertNoError(t, err)
	if got := f.balance(t, f.cashMYR); got != start {
		t.Errorf("old account = %d, want %d", got, start)
	}
	if got := f.balance(t, other); got != 900 {
		t.Errorf("new account = %d, want 900", got)
	}

	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, div.ID, UpdateTradeParams{Date: nativeDay, Amount: 1000, Fee: 100})
	testutil.AssertNoError(t, err)
	if got := f.balance(t, other); got != 0 {
		t.Errorf("new account after dropping the leg = %d", got)
	}
	if legs := cashLegOf(t, f, div.ID); len(legs) != 0 {
		t.Errorf("cash legs = %+v", legs)
	}
}

func TestUpdateInvestmentTransaction_Split(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	_, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000})
	testutil.AssertNoError(t, err)
	split, err := f.svc.RecordSplit(f.user.ID, inv.ID, nativeDay.Add(time.Hour), 2, "")
	testutil.AssertNoError(t, err)

	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, split.ID, UpdateTradeParams{Date: nativeDay.Add(time.Hour), SplitRatio: 3})
	testutil.AssertNoError(t, err)
	if got := f.reload(t, inv); got.Quantity != 30 || got.CostBasis != 10000 {
		t.Errorf("holding = %+v", got)
	}
	var s models.InvestmentTransaction
	testutil.AssertNoError(t, f.db.First(&s, "id = ?", split.ID).Error)
	if s.Quantity != 10 || s.SplitRatio != 3 {
		t.Errorf("split row = qty %v ratio %v", s.Quantity, s.SplitRatio)
	}
	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, split.ID, UpdateTradeParams{Date: nativeDay, SplitRatio: 0})
	testutil.AssertAppError(t, err, "INVALID_INPUT")
}

func TestLegacyCashLeg_FoundByDescription(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	start := f.balance(t, f.cashMYR)
	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	// Make the cash leg look like one written before migration 000058.
	testutil.AssertNoError(t, f.db.Model(&models.Transaction{}).Where("investment_transaction_id = ?", buy.ID).
		Update("investment_transaction_id", nil).Error)

	page, err := f.svc.GetInvestmentTransactions(f.user.ID, inv.ID, pagination.PageRequest{})
	testutil.AssertNoError(t, err)
	if got := page.Data[0]; got.CashAccountID == nil || *got.CashAccountID != f.cashMYR.ID || *got.CashAmount != 10000 {
		t.Fatalf("legacy cash leg not found: %v", got.CashAccountID)
	}

	testutil.AssertNoError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, inv.ID, buy.ID))
	if got := f.balance(t, f.cashMYR); got != start {
		t.Errorf("cash = %d, want %d", got, start)
	}
}

func TestLegacyCashLeg_AmbiguousIsLeftAlone(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	other := testutil.CreateTestCashAccountWithBalance(t, f.db, f.user.ID, 1_000_000)
	testutil.AssertNoError(t, f.db.Model(other).Update("currency", "MYR").Error)

	a, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	_, err = f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: other.ID})
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, f.db.Model(&models.Transaction{}).Where("investment_transaction_id IS NOT NULL").
		Update("investment_transaction_id", nil).Error)
	before, beforeOther := f.balance(t, f.cashMYR), f.balance(t, other)

	// Two identical trades paid from different accounts: which cash row
	// belongs to which is unknown, so neither is touched.
	testutil.AssertNoError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, inv.ID, a.ID))
	if f.balance(t, f.cashMYR) != before || f.balance(t, other) != beforeOther {
		t.Errorf("an ambiguous legacy cash leg was reversed")
	}
}

func TestDeleteInvestmentTransaction_TotalInvestedIgnoresDeleted(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	_, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000})
	testutil.AssertNoError(t, err)
	typo, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 100, PricePerUnit: 1000})
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, inv.ID, typo.ID))

	got, err := f.svc.GetInvestmentByID(f.user.ID, inv.ID)
	testutil.AssertNoError(t, err)
	if got.TotalInvested != 10000 {
		t.Errorf("total_invested = %d, want 10000", got.TotalInvested)
	}
}

func TestLegacyCashLeg_SameSecurityInAnotherHoldingIsAmbiguous(t *testing.T) {
	f := newNativeFixture(t, false)
	a := f.holding(t, f.myr)
	otherAcct := testutil.CreateTestInvestmentAccount(t, f.db, f.user.ID)
	b := &models.Investment{AccountID: otherAcct.ID, SecurityID: f.myr.ID}
	testutil.AssertNoError(t, f.db.Create(b).Error)

	// Paid from cash in holding A; no cash account in holding B. Same day, same
	// quantity, so both would have written the same description.
	_, err := f.svc.RecordBuy(f.user.ID, a.ID, BuyParams{Date: nativeDay, Quantity: 100, PricePerUnit: 950, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	bBuy, err := f.svc.RecordBuy(f.user.ID, b.ID, BuyParams{Date: nativeDay, Quantity: 100, PricePerUnit: 950})
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, f.db.Model(&models.Transaction{}).Where("investment_transaction_id IS NOT NULL").
		Update("investment_transaction_id", nil).Error)
	before := f.balance(t, f.cashMYR)

	page, err := f.svc.GetInvestmentTransactions(f.user.ID, b.ID, pagination.PageRequest{})
	testutil.AssertNoError(t, err)
	if page.Data[0].CashAccountID != nil {
		t.Errorf("holding B shows holding A's cash leg")
	}
	testutil.AssertNoError(t, f.svc.DeleteInvestmentTransaction(f.user.ID, b.ID, bBuy.ID))
	if got := f.balance(t, f.cashMYR); got != before {
		t.Errorf("deleting B reversed A's cash: %d -> %d", before, got)
	}
}

func TestUpdateInvestmentTransaction_KeepsDeactivatedAccount(t *testing.T) {
	f := newNativeFixture(t, false)
	inv := f.holding(t, f.myr)
	start := f.balance(t, f.cashMYR)
	buy, err := f.svc.RecordBuy(f.user.ID, inv.ID, BuyParams{Date: nativeDay, Quantity: 10, PricePerUnit: 1000, FundingAccountID: f.cashMYR.ID})
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, f.db.Model(f.cashMYR).Update("is_active", false).Error)

	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, buy.ID, UpdateTradeParams{
		Date: nativeDay, Quantity: 10, PricePerUnit: 900, AccountID: f.cashMYR.ID,
	})
	testutil.AssertNoError(t, err)
	if got := f.balance(t, f.cashMYR); got != start-9000 {
		t.Errorf("cash = %d, want %d", got, start-9000)
	}

	// A different inactive account is still refused.
	other := testutil.CreateTestCashAccountWithBalance(t, f.db, f.user.ID, 0)
	testutil.AssertNoError(t, f.db.Model(other).Updates(map[string]interface{}{"currency": "MYR", "is_active": false}).Error)
	_, err = f.svc.UpdateInvestmentTransaction(f.user.ID, inv.ID, buy.ID, UpdateTradeParams{
		Date: nativeDay, Quantity: 10, PricePerUnit: 900, AccountID: other.ID,
	})
	if err == nil {
		t.Errorf("moving the cash leg to an inactive account was allowed")
	}
}
