package services

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

// UpdateTradeParams replaces the editable fields of an investment transaction.
// Its type cannot change; only the fields of that type are read:
//   - buy / sell: Quantity, PricePerUnit, Fee, AccountID, AccountAmount, FXRate
//   - dividend: Amount (gross), Fee (withholding), DividendType, AccountID, AccountAmount, FXRate
//   - split: SplitRatio
//
// Date and Notes apply to every type. AccountID is the cash account that funds
// a buy or receives a sell or dividend ("" = no cash leg) and AccountAmount the
// amount moved there in that account's currency, as funding_amount /
// deposit_amount on the record endpoints. With FXRate empty and the date,
// total and cash leg unchanged, the stored FX rate is kept.
type UpdateTradeParams struct {
	Date          time.Time
	Quantity      float64
	PricePerUnit  int64
	Amount        int64
	Fee           int64
	Notes         string
	DividendType  string
	SplitRatio    float64
	AccountID     string
	AccountAmount int64
	FXRate        string
}

// cashLegDescription is the description RecordBuy/RecordSell/RecordDividend
// gave the cash leg of row, used to find legacy cash legs that predate the
// investment_transaction_id link.
func cashLegDescription(row *models.InvestmentTransaction, securityName string) string {
	switch row.Type {
	case models.InvestmentTransactionBuy:
		return fmt.Sprintf("Investment purchase: %g %s", row.Quantity, securityName)
	case models.InvestmentTransactionSell:
		return fmt.Sprintf("Sale proceeds: %g %s", row.Quantity, securityName)
	case models.InvestmentTransactionDividend:
		return fmt.Sprintf("Dividend: %s", securityName)
	}
	return ""
}

// cashLegNative is the part of row, in the security's currency, its cash leg
// settles: the buy cost, the net sell proceeds or the net dividend.
func cashLegNative(row *models.InvestmentTransaction) int64 {
	if row.Type == models.InvestmentTransactionDividend {
		return row.TotalAmount - row.Fee
	}
	return row.TotalAmount
}

// cashLegReversal is the balance effect that undoes the cash leg of row.
func cashLegReversal(row *models.InvestmentTransaction) models.TransactionType {
	if row.Type == models.InvestmentTransactionBuy {
		return models.TransactionTypeIncome // the buy took money out
	}
	return models.TransactionTypeExpense
}

// cashLegsFor finds the cash leg of each row of investment (Security loaded).
// A row written since migration 000058 is linked by investment_transaction_id.
// An older one is matched to an unlinked system cash row of the user with the
// same date and the description its record path wrote (and, when the account
// is in the security's currency, the same amount). A legacy match is accepted
// only when it is unambiguous: one candidate, or several that are
// interchangeable (same account and amount, e.g. two identical buys). Imported
// rows never match: their cash is booked by the importer with an import_ref.
func cashLegsFor(db *gorm.DB, userID string, investment *models.Investment, rows []models.InvestmentTransaction) (map[string]*models.Transaction, error) {
	out := make(map[string]*models.Transaction, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}

	var linked []models.Transaction
	if err := db.Preload("Account").
		Where("user_id = ? AND investment_transaction_id IN ?", userID, ids).
		Find(&linked).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	for i := range linked {
		out[*linked[i].InvestmentTransactionID] = &linked[i]
	}

	// Legacy lookup for the rows still without a leg.
	var dates []time.Time
	var descs []string
	var pending []*models.InvestmentTransaction
	for i := range rows {
		row := &rows[i]
		desc := cashLegDescription(row, investment.Security.Name)
		if _, ok := out[row.ID]; ok || desc == "" {
			continue
		}
		pending = append(pending, row)
		dates = append(dates, row.Date)
		descs = append(descs, desc)
	}
	if len(pending) == 0 {
		return out, nil
	}
	var candidates []models.Transaction
	if err := db.Preload("Account").
		Where("user_id = ? AND type = ? AND source = ? AND import_ref IS NULL AND investment_transaction_id IS NULL",
			userID, models.TransactionTypeInvestment, models.TransactionSourceSystem).
		Where("date IN ? AND description IN ?", dates, descs).
		Order("created_at ASC, id ASC").
		Find(&candidates).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	secCcy := normalizeCode(investment.Security.Currency)
	claimed := make(map[string]bool, len(candidates))
	sortTradesForClaim(pending)
	for _, row := range pending {
		desc := cashLegDescription(row, investment.Security.Name)
		var match []*models.Transaction
		for i := range candidates {
			c := &candidates[i]
			if claimed[c.ID] || !c.Date.Equal(row.Date) || c.Description != desc {
				continue
			}
			if normalizeCode(c.Account.Currency) == secCcy && c.Amount != cashLegNative(row) {
				continue
			}
			match = append(match, c)
		}
		if len(match) == 0 {
			continue
		}
		interchangeable := true
		for _, m := range match[1:] {
			if m.AccountID != match[0].AccountID || m.Amount != match[0].Amount {
				interchangeable = false
				break
			}
		}
		if !interchangeable {
			continue
		}
		claimed[match[0].ID] = true
		out[row.ID] = match[0]
	}
	return out, nil
}

// sortTradesForClaim orders rows by (created_at, id) so identical legacy trades
// claim identical cash rows in a stable order.
func sortTradesForClaim(rows []*models.InvestmentTransaction) {
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		}
		return rows[i].ID < rows[j].ID
	})
}

// editableColumns lists the investment_transactions columns an edit of a
// transaction of type typ rewrites. A split keeps its stored quantity (the
// position it applied to, informational only).
func editableColumns(typ models.InvestmentTransactionType) []string {
	switch typ {
	case models.InvestmentTransactionSplit:
		return []string{"date", "split_ratio", "notes"}
	case models.InvestmentTransactionDividend:
		return []string{"date", "total_amount", "fee", "notes", "fx_rate_to_base", "base_amount", "dividend_type"}
	default:
		return []string{"date", "quantity", "price_per_unit", "total_amount", "fee", "notes", "fx_rate_to_base", "base_amount"}
	}
}

// attachCashLegs fills the response-only cash_account_id / cash_amount fields.
func attachCashLegs(db *gorm.DB, userID string, investment *models.Investment, rows []models.InvestmentTransaction) error {
	legs, err := cashLegsFor(db, userID, investment, rows)
	if err != nil {
		return err
	}
	for i := range rows {
		if leg, ok := legs[rows[i].ID]; ok {
			id, amount := leg.AccountID, leg.Amount
			rows[i].CashAccountID, rows[i].CashAmount = &id, &amount
		}
	}
	return nil
}

// loadTrade reads one investment transaction of investmentID.
func loadTrade(db *gorm.DB, investmentID, transactionID string) (*models.InvestmentTransaction, error) {
	var row models.InvestmentTransaction
	if err := db.Where("id = ? AND investment_id = ?", transactionID, investmentID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrInvestmentTransactionNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &row, nil
}

// reverseCashLeg soft-deletes a cash leg and undoes its balance effect on its
// (row-locked) account. The account may since have been deactivated: undoing
// a mistake must still be possible.
func (s *investmentService) reverseCashLeg(tx *gorm.DB, userID string, row *models.InvestmentTransaction, leg *models.Transaction) error {
	accounts, err := lockAccounts(tx, userID, []string{leg.AccountID}, false)
	if err != nil {
		return err
	}
	acct := accounts[leg.AccountID]
	if err := tx.Delete(&models.Transaction{}, "id = ?", leg.ID).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return s.accountService.ApplyBalanceDelta(tx, acct.ID, acct.Type, cashLegReversal(row), leg.Amount)
}

// DeleteInvestmentTransaction deletes a buy, sell, dividend or split and its
// cash leg (the cash account's balance is restored exactly), then rebuilds the
// holding from the remaining transactions. A deletion that would leave a later
// sell larger than the position it sells from is refused with
// INSUFFICIENT_SHARES and changes nothing.
func (s *investmentService) DeleteInvestmentTransaction(userID, investmentID, transactionID string) error {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockInvestment(tx, investmentID); err != nil {
			return err
		}
		row, err := loadTrade(tx, investmentID, transactionID)
		if err != nil {
			return err
		}
		if row.Type == models.InvestmentTransactionTransfer {
			return apperrors.WithMessage(apperrors.ErrInvalidInput, "transfer transactions cannot be deleted")
		}
		legs, err := cashLegsFor(tx, userID, investment, []models.InvestmentTransaction{*row})
		if err != nil {
			return err
		}
		if leg, ok := legs[row.ID]; ok {
			if err := s.reverseCashLeg(tx, userID, row, leg); err != nil {
				return err
			}
		}
		if err := tx.Delete(&models.InvestmentTransaction{}, "id = ?", row.ID).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
		return rebuildInvestment(tx, investmentID)
	})
}

// UpdateInvestmentTransaction corrects a buy, sell, dividend or split in place
// (its id and type stay) with the same validation and pricing as recording it,
// replaces its cash leg (reversing the old one, moving the new one) and
// rebuilds the holding, so every later sell's realized gain is recomputed. An
// edit that would leave a sell larger than the position it sells from is
// refused with INSUFFICIENT_SHARES and changes nothing.
func (s *investmentService) UpdateInvestmentTransaction(userID, investmentID, transactionID string, p UpdateTradeParams) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}
	old, err := loadTrade(s.db, investmentID, transactionID)
	if err != nil {
		return nil, err
	}
	legs, err := cashLegsFor(s.db, userID, investment, []models.InvestmentTransaction{*old})
	if err != nil {
		return nil, err
	}
	oldLeg := legs[old.ID]

	keep := &keptRate{date: old.Date, total: old.TotalAmount, rate: old.FXRateToBase, base: old.BaseAmount}
	if oldLeg != nil {
		keep.accountID, keep.cashAmount = oldLeg.AccountID, oldLeg.Amount
	}

	pt, err := s.prepareEdit(userID, investment, old.Type, p, keep)
	if err != nil {
		return nil, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if _, txErr := lockInvestment(tx, investmentID); txErr != nil {
			return txErr
		}
		// Re-read under the lock: the row or its cash leg may have changed
		// since the read above.
		cur, txErr := loadTrade(tx, investmentID, transactionID)
		if txErr != nil {
			return txErr
		}
		curLegs, txErr := cashLegsFor(tx, userID, investment, []models.InvestmentTransaction{*cur})
		if txErr != nil {
			return txErr
		}
		curLeg := curLegs[cur.ID]

		if txErr := tx.Model(&models.InvestmentTransaction{}).Where("id = ?", cur.ID).
			Select(editableColumns(cur.Type)).
			Updates(&pt.row).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}
		if txErr := s.replaceCashLeg(tx, userID, cur, curLeg, pt); txErr != nil {
			return txErr
		}
		return rebuildInvestment(tx, investmentID)
	})
	if err != nil {
		return nil, err
	}

	updated, err := loadTrade(s.db, investmentID, transactionID)
	if err != nil {
		return nil, err
	}
	rows := []models.InvestmentTransaction{*updated}
	if err := attachCashLegs(s.db, userID, investment, rows); err != nil {
		return nil, err
	}
	rows[0].Currency = normalizeCode(investment.Security.Currency)
	return &rows[0], nil
}

// replaceCashLeg swaps the cash leg of cur for the prepared one. When both
// move the same amount on the same account the old row is kept (its date and
// description follow the edit, and a legacy row gets linked) so its balance
// effect, category and attachments stay untouched.
func (s *investmentService) replaceCashLeg(tx *gorm.DB, userID string, cur *models.InvestmentTransaction, leg *models.Transaction, pt *preparedTrade) error {
	if leg != nil && pt.cash != nil && leg.AccountID == pt.cash.ID && leg.Amount == pt.cashAmount {
		id := cur.ID
		if err := tx.Model(&models.Transaction{}).Where("id = ?", leg.ID).Updates(map[string]interface{}{
			"date":                      pt.row.Date,
			"description":               pt.cashDesc,
			"investment_transaction_id": &id,
		}).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
		return nil
	}
	if leg != nil {
		if err := s.reverseCashLeg(tx, userID, cur, leg); err != nil {
			return err
		}
	}
	return s.writeCashLeg(tx, userID, pt, cur.ID)
}

// prepareEdit validates and prices the replacement of a transaction of type typ.
func (s *investmentService) prepareEdit(userID string, investment *models.Investment, typ models.InvestmentTransactionType, p UpdateTradeParams, keep *keptRate) (*preparedTrade, error) {
	if p.Date.IsZero() {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "date is required")
	}
	if p.Fee < 0 {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "fee must not be negative")
	}
	switch typ {
	case models.InvestmentTransactionBuy, models.InvestmentTransactionSell:
		if p.Quantity <= 0 {
			return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "quantity must be greater than zero")
		}
		if p.PricePerUnit <= 0 {
			return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "price_per_unit must be greater than zero")
		}
		if typ == models.InvestmentTransactionBuy {
			return s.prepareBuy(userID, investment, BuyParams{
				Date: p.Date, Quantity: p.Quantity, PricePerUnit: p.PricePerUnit, Fee: p.Fee, Notes: p.Notes,
				FundingAccountID: p.AccountID, FundingAmount: p.AccountAmount, FXRate: p.FXRate,
			}, keep)
		}
		return s.prepareSell(userID, investment, SellParams{
			Date: p.Date, Quantity: p.Quantity, PricePerUnit: p.PricePerUnit, Fee: p.Fee, Notes: p.Notes,
			DepositAccountID: p.AccountID, DepositAmount: p.AccountAmount, FXRate: p.FXRate,
		}, keep)
	case models.InvestmentTransactionDividend:
		if p.Amount <= 0 {
			return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "amount must be greater than zero")
		}
		return s.prepareDividend(userID, investment, DividendParams{
			Date: p.Date, Amount: p.Amount, Fee: p.Fee, DividendType: p.DividendType, Notes: p.Notes,
			DepositAccountID: p.AccountID, DepositAmount: p.AccountAmount, FXRate: p.FXRate,
		}, keep)
	case models.InvestmentTransactionSplit:
		if p.SplitRatio <= 0 {
			return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "split_ratio must be greater than zero")
		}
		if p.AccountID != "" {
			return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "a split moves no cash")
		}
		return &preparedTrade{row: models.InvestmentTransaction{
			Type: typ, Date: p.Date, SplitRatio: p.SplitRatio, Notes: p.Notes,
		}}, nil
	}
	return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, fmt.Sprintf("%s transactions cannot be edited", typ))
}
