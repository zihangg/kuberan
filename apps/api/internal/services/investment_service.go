package services

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/fx"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// InvestmentStatus filters investments by position state.
type InvestmentStatus string

const (
	InvestmentStatusOpen   InvestmentStatus = "open"   // quantity > 0
	InvestmentStatusClosed InvestmentStatus = "closed" // quantity = 0
	InvestmentStatusAll    InvestmentStatus = "all"    // no filter
)

// applyStatusFilter applies the quantity clause that corresponds to the given status.
// Unknown values fall back to InvestmentStatusOpen so callers that omit the filter
// keep their original behaviour.
func applyStatusFilter(db *gorm.DB, status InvestmentStatus) *gorm.DB {
	switch status {
	case InvestmentStatusClosed:
		return db.Where("quantity = 0")
	case InvestmentStatusAll:
		return db
	default:
		return db.Where("quantity > 0")
	}
}

// uniqueStrings returns ids without duplicates, preserving first-seen order.
func uniqueStrings(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// getTotalInvestedByID returns the sum of buy transaction TotalAmount per investment ID.
// Used to expose the original capital deployed for closed positions, where CostBasis is
// driven to zero by proportional sell-side reductions.
func getTotalInvestedByID(db *gorm.DB, investmentIDs []string) (map[string]int64, error) {
	if len(investmentIDs) == 0 {
		return map[string]int64{}, nil
	}

	type row struct {
		InvestmentID string
		Total        int64
	}
	var rows []row

	if err := db.Table("investment_transactions").
		Select("investment_id, COALESCE(SUM(total_amount), 0) AS total").
		Where("investment_id IN ? AND type = ? AND deleted_at IS NULL", investmentIDs, models.InvestmentTransactionBuy).
		Group("investment_id").
		Scan(&rows).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.InvestmentID] = r.Total
	}
	return result, nil
}

// investmentListOrder gives paginated investment lists a deterministic order.
const investmentListOrder = "created_at ASC, id ASC"

// investmentService handles investment-related business logic.
type investmentService struct {
	db             *gorm.DB
	accountService AccountServicer
	fxService      FXServicer
}

// NewInvestmentService creates a new InvestmentServicer. fxs converts native
// security prices into the user's base currency; production wiring always
// passes the FX service. A nil fxs is allowed in tests only: then only
// same-currency conversion succeeds.
func NewInvestmentService(db *gorm.DB, accountService AccountServicer, fxs FXServicer) InvestmentServicer {
	return &investmentService{db: db, accountService: accountService, fxService: fxs}
}

// populatePrices fills the price fields of the given investments: the latest
// native price, and CurrentPrice converted to the user's base currency (a unit
// price conversion, which is fine here because it is display only). With no
// FX rate CurrentPrice is 0 and FXMissing is set. Security must be loaded.
func (s *investmentService) populatePrices(userID string, investments []*models.Investment) error {
	if len(investments) == 0 {
		return nil
	}
	base, err := userBaseCurrency(s.db, userID)
	if err != nil {
		return err
	}
	secIDs := make([]string, len(investments))
	for i, inv := range investments {
		secIDs[i] = inv.SecurityID
	}
	prices, err := latestNativePrices(s.db, secIDs)
	if err != nil {
		return err
	}
	v, err := newValuer(s.fxService, base, time.Now(), distinctPriceCurrencies(prices))
	if err != nil {
		return err
	}
	for _, inv := range investments {
		inv.Currency = normalizeCode(inv.Security.Currency)
		p, ok := prices[inv.SecurityID]
		if !ok {
			inv.PriceCurrency = inv.Currency
			continue
		}
		inv.CurrentPriceNative = p.Price
		inv.PriceCurrency = p.Currency
		converted, quote, ok := v.Convert(p.Price, p.Currency)
		if !ok {
			inv.CurrentPrice = 0
			inv.FXMissing = true
			continue
		}
		inv.CurrentPrice = converted
		if quote != nil {
			inv.FXRate = quote.Rate.String()
			inv.FXStale = quote.Stale
		}
	}
	return nil
}

// investmentPtrs returns pointers to the elements of investments.
func investmentPtrs(investments []models.Investment) []*models.Investment {
	out := make([]*models.Investment, len(investments))
	for i := range investments {
		out[i] = &investments[i]
	}
	return out
}

// AddInvestment adds a new investment holding to an investment account. The
// price and fee are in the security's currency; the initial buy's base-currency
// figures use the trade FX rate (an explicit p.FXRate, else 1 for a security in
// the base currency, else the market rate on the trade date).
func (s *investmentService) AddInvestment(userID string, p AddInvestmentParams) (*models.Investment, error) {
	// Verify account exists, belongs to user, and is an investment account
	account, err := s.accountService.GetAccountByID(userID, p.AccountID)
	if err != nil {
		return nil, err
	}
	if account.Type != models.AccountTypeInvestment {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "Account is not an investment account")
	}

	// Verify security exists
	var security models.Security
	if err := s.db.Where("id = ?", p.SecurityID).First(&security).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSecurityNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Apply defaults for optional fields
	txDate := time.Now()
	if p.Date != nil {
		txDate = *p.Date
	}
	txNotes := "Initial purchase"
	if p.Notes != "" {
		txNotes = p.Notes
	}

	explicit, err := parseOptionalRate(p.FXRate)
	if err != nil {
		return nil, err
	}
	gross, err := lineAmount(p.Quantity, p.PurchasePrice)
	if err != nil {
		return nil, err
	}
	totalAmount := gross + p.Fee

	base, err := userBaseCurrency(s.db, userID)
	if err != nil {
		return nil, err
	}
	rate, err := s.tradeRate(base, normalizeCode(security.Currency), txDate, explicit, nil)
	if err != nil {
		return nil, err
	}
	baseAmount, err := baseAmountFor(totalAmount, rate)
	if err != nil {
		return nil, err
	}

	var st holdingState
	if _, err := st.apply(tradeEvent{
		Type: models.InvestmentTransactionBuy, Quantity: p.Quantity, TotalAmount: totalAmount, BaseAmount: baseAmount,
	}); err != nil {
		return nil, err
	}

	investment := &models.Investment{
		AccountID:     p.AccountID,
		SecurityID:    p.SecurityID,
		Quantity:      st.Quantity,
		CostBasis:     st.CostBasis,
		CostBasisBase: st.CostBasisBase,
		WalletAddress: p.WalletAddress,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if txErr := tx.Create(investment).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		// Create initial buy transaction
		invTx := &models.InvestmentTransaction{
			InvestmentID: investment.ID,
			Type:         models.InvestmentTransactionBuy,
			Date:         txDate,
			Quantity:     p.Quantity,
			PricePerUnit: p.PurchasePrice,
			TotalAmount:  totalAmount,
			Fee:          p.Fee,
			Notes:        txNotes,
			FXRateToBase: &rate,
			BaseAmount:   &baseAmount,
		}
		if txErr := tx.Create(invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Populate current price from security_prices for the response
	investment.Security = security
	if err := s.populatePrices(userID, []*models.Investment{investment}); err != nil {
		return nil, err
	}
	return investment, nil
}

// GetAccountInvestments returns a paginated list of investments for an account.
func (s *investmentService) GetAccountInvestments(userID, accountID string, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
	// Verify account exists and belongs to user (deactivated accounts stay readable)
	if _, err := s.accountService.GetAccountByIDIncludingInactive(userID, accountID); err != nil {
		return nil, err
	}

	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.Investment{}).Where("account_id = ? AND quantity > 0", accountID)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var investments []models.Investment
	if err := s.db.Preload("Security").Where("account_id = ? AND quantity > 0", accountID).
		Order(investmentListOrder).
		Scopes(pagination.Paginate(page)).Find(&investments).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Batch populate current prices from security_prices
	invIDs := make([]string, 0, len(investments))
	for i := range investments {
		invIDs = append(invIDs, investments[i].ID)
	}
	if err := s.populatePrices(userID, investmentPtrs(investments)); err != nil {
		return nil, err
	}
	invested, err := getTotalInvestedByID(s.db, invIDs)
	if err != nil {
		return nil, err
	}
	for i := range investments {
		investments[i].TotalInvested = invested[investments[i].ID]
	}

	result := pagination.NewPageResponse(investments, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// GetAllInvestments returns a paginated list of investments across all active
// investment accounts for the given user, filtered by status (open/closed/all).
func (s *investmentService) GetAllInvestments(userID string, status InvestmentStatus, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
	page.Defaults()

	// Find all active investment account IDs for the user
	var accountIDs []string
	if err := s.db.Model(&models.Account{}).
		Where("user_id = ? AND type = ? AND is_active = ?", userID, models.AccountTypeInvestment, true).
		Pluck("id", &accountIDs).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	if len(accountIDs) == 0 {
		empty := pagination.NewPageResponse([]models.Investment{}, page.Page, page.PageSize, 0)
		return &empty, nil
	}

	var totalItems int64
	base := applyStatusFilter(
		s.db.Model(&models.Investment{}).Where("account_id IN ?", accountIDs),
		status,
	)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var investments []models.Investment
	query := applyStatusFilter(
		s.db.Preload("Security").Preload("Account").Where("account_id IN ?", accountIDs),
		status,
	)
	if err := query.Order(investmentListOrder).Scopes(pagination.Paginate(page)).Find(&investments).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Batch populate current prices from security_prices
	invIDs := make([]string, 0, len(investments))
	for i := range investments {
		invIDs = append(invIDs, investments[i].ID)
	}
	if err := s.populatePrices(userID, investmentPtrs(investments)); err != nil {
		return nil, err
	}
	invested, err := getTotalInvestedByID(s.db, invIDs)
	if err != nil {
		return nil, err
	}
	for i := range investments {
		investments[i].TotalInvested = invested[investments[i].ID]
	}

	result := pagination.NewPageResponse(investments, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// GetInvestmentByID returns an investment if the parent account belongs to the user.
func (s *investmentService) GetInvestmentByID(userID, investmentID string) (*models.Investment, error) {
	var investment models.Investment
	if err := s.db.Preload("Account").Preload("Security").Where("id = ?", investmentID).First(&investment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrInvestmentNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Verify account belongs to user
	if investment.Account.UserID != userID {
		return nil, apperrors.ErrInvestmentNotFound
	}

	// Populate current price from security_prices
	if err := s.populatePrices(userID, []*models.Investment{&investment}); err != nil {
		return nil, err
	}

	invested, err := getTotalInvestedByID(s.db, []string{investment.ID})
	if err != nil {
		return nil, err
	}
	investment.TotalInvested = invested[investment.ID]

	return &investment, nil
}

// GetPortfolio returns an aggregated portfolio summary across all investment accounts,
// valued in the user's base currency. Each holding is valued in its native currency
// first (quantity × price) and the line value is converted once. Holdings with no
// FX rate are left out of the totals and listed in Unconverted.
//
// Cost basis is the stored base-currency cost (frozen at each buy's rate). The
// unrealized gain splits into a price return (the native gain converted at
// today's rate) and an FX return (today's rate on the native cost versus the
// frozen base cost); per holding they add up to value − cost basis within one
// cent of rounding.
func (s *investmentService) GetPortfolio(userID string) (*PortfolioSummary, error) {
	base, err := userBaseCurrency(s.db, userID)
	if err != nil {
		return nil, err
	}

	// Get all investment accounts for the user
	var accounts []models.Account
	if err := s.db.Where("user_id = ? AND type = ? AND is_active = ?", userID, models.AccountTypeInvestment, true).
		Find(&accounts).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	accountIDs := make([]string, len(accounts))
	for i := range accounts {
		accountIDs[i] = accounts[i].ID
	}

	summary := &PortfolioSummary{
		HoldingsByType: make(map[models.AssetType]TypeSummary),
		Currency:       base,
		Unconverted:    []UnconvertedValue{},
	}

	if len(accountIDs) == 0 {
		return summary, nil
	}

	// Get all investments across those accounts with Security preloaded
	var investments []models.Investment
	if err := s.db.Preload("Security").Where("account_id IN ?", accountIDs).Find(&investments).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Batch fetch live prices from security_prices
	secIDs := make([]string, 0, len(investments))
	for i := range investments {
		secIDs = append(secIDs, investments[i].SecurityID)
	}
	prices, err := latestNativePrices(s.db, secIDs)
	if err != nil {
		return nil, err
	}
	// Quote every currency an open holding is valued or costed in: the price
	// currencies, plus the security currencies (cost basis converts even for a
	// holding with no price yet).
	currencies := distinctPriceCurrencies(prices)
	for i := range investments {
		if investments[i].Quantity > 0 {
			currencies = append(currencies, normalizeCode(investments[i].Security.Currency))
		}
	}
	v, err := newValuer(s.fxService, base, time.Now(), sortedUnique(currencies))
	if err != nil {
		return nil, err
	}

	unconverted := map[string]int64{}
	for i := range investments {
		inv := &investments[i]

		// Always include realized G/L from all positions (open + closed)
		summary.TotalRealizedGainLoss += inv.RealizedGainLossBase

		// Only include open positions in holdings counts, values, and cost basis
		if inv.Quantity <= 0 {
			continue
		}
		cur := normalizeCode(inv.Security.Currency)
		var native int64
		if p, priced := prices[inv.SecurityID]; priced {
			native = valueCents(inv.Quantity, p.Price)
			cur = p.Currency
		}
		value, quote, ok := v.Convert(native, cur)
		priceReturn, _, okPrice := v.Convert(native-inv.CostBasis, cur)
		costNow, _, okCost := v.Convert(inv.CostBasis, cur)
		if !ok || !okPrice || !okCost {
			unconverted[cur] += native
			continue
		}
		if quote != nil && quote.Stale {
			summary.FXStale = true
		}
		summary.TotalValue += value
		summary.TotalCostBasis += inv.CostBasisBase
		summary.TotalPriceReturn += priceReturn
		summary.TotalFXReturn += costNow - inv.CostBasisBase

		ts := summary.HoldingsByType[inv.Security.AssetType]
		ts.Value += value
		ts.Count++
		summary.HoldingsByType[inv.Security.AssetType] = ts
	}

	for _, c := range sortedUnique(mapKeys(unconverted)) {
		summary.Unconverted = append(summary.Unconverted, UnconvertedValue{Currency: c, Value: unconverted[c]})
	}

	summary.TotalGainLoss = summary.TotalValue - summary.TotalCostBasis
	if summary.TotalCostBasis > 0 {
		summary.GainLossPct = float64(summary.TotalGainLoss) / float64(summary.TotalCostBasis) * 100
	}

	return summary, nil
}

// mapKeys returns the keys of m.
func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// cashLegRequest names the optional cash account that settles a trade.
type cashLegRequest struct {
	accountID string
	amount    int64  // caller-stated amount in the account's currency; 0 = not given
	field     string // request field carrying amount ("funding_amount" / "deposit_amount")
	native    int64  // the amount, in the security's currency, that the leg settles
	// keepAccountID is the account an edited trade already used: it stays
	// usable even if it has since been deactivated.
	keepAccountID string
}

// resolveCashLeg validates the cash account of a trade and returns it with the
// amount to move in the ACCOUNT's currency. With no account it returns nil. In
// the security's currency the amount is the native one (a stated amount must
// match it); otherwise the caller must state the amount, else FX_AMOUNT_REQUIRED
// is returned with a market-rate suggestion.
func (s *investmentService) resolveCashLeg(userID string, req cashLegRequest, secCcy string, date time.Time) (*models.Account, int64, error) {
	if req.amount < 0 {
		return nil, 0, apperrors.WithMessage(apperrors.ErrInvalidInput, req.field+" must be greater than zero")
	}
	if req.accountID == "" {
		if req.amount != 0 {
			return nil, 0, apperrors.WithMessage(apperrors.ErrInvalidInput, req.field+" requires an account")
		}
		return nil, 0, nil
	}
	lookup := s.accountService.GetAccountByID
	if req.keepAccountID != "" && req.accountID == req.keepAccountID {
		lookup = s.accountService.GetAccountByIDIncludingInactive
	}
	account, err := lookup(userID, req.accountID)
	if err != nil {
		return nil, 0, err
	}
	if account.Type != models.AccountTypeCash {
		kind := "funding"
		if req.field == "deposit_amount" {
			kind = "deposit"
		}
		return nil, 0, apperrors.WithMessage(apperrors.ErrInvalidInput, kind+" account must be a cash account")
	}
	cur := normalizeCode(account.Currency)
	if cur == secCcy {
		if req.amount != 0 && req.amount != req.native {
			return nil, 0, apperrors.WithMessage(apperrors.ErrInvalidInput,
				fmt.Sprintf("%s must equal %d when the account is in the security's currency", req.field, req.native))
		}
		return account, req.native, nil
	}
	if req.amount == 0 {
		return nil, 0, fxAmountRequiredError(s.fxService, secCcy, cur, req.native, date, req.field)
	}
	return account, req.amount, nil
}

// applyCashLeg writes the cash transaction row and moves the (row-locked)
// account's balance by amount, in the account's own currency. With
// requireActive the account must be active (an edit keeping the account its
// trade already used passes false). investmentTxID
// links the row to the trade it settles, so the trade can later be edited or
// deleted together with its cash.
func (s *investmentService) applyCashLeg(tx *gorm.DB, userID, accountID string, txnType models.TransactionType, amount int64, description string, date time.Time, investmentTxID string, requireActive bool) error {
	accounts, err := lockAccounts(tx, userID, []string{accountID}, requireActive)
	if err != nil {
		return err
	}
	acct := accounts[accountID]
	cashTx := models.Transaction{
		UserID:                  userID,
		AccountID:               accountID,
		Type:                    models.TransactionTypeInvestment,
		Amount:                  amount,
		Description:             description,
		Date:                    date,
		Source:                  models.TransactionSourceSystem,
		InvestmentTransactionID: &investmentTxID,
	}
	if err := tx.Create(&cashTx).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return s.accountService.ApplyBalanceDelta(tx, acct.ID, acct.Type, txnType, amount)
}

// preparedTrade is a validated and priced buy, sell, dividend or split, ready
// to be written: the investment transaction row (without ID or InvestmentID)
// and its cash leg. cash is nil when the trade moves no cash.
type preparedTrade struct {
	row        models.InvestmentTransaction
	cash       *models.Account
	cashAmount int64                  // in the cash account's currency
	cashType   models.TransactionType // the cash leg's balance effect (expense = money out)
	cashDesc   string
	// keepAccountID is the account an edited trade already used ("" when
	// recording): it may be inactive.
	keepAccountID string
}

// writeCashLeg writes the prepared trade's cash leg, linked to investmentTxID.
func (s *investmentService) writeCashLeg(tx *gorm.DB, userID string, pt *preparedTrade, investmentTxID string) error {
	if pt.cash == nil {
		return nil
	}
	return s.applyCashLeg(tx, userID, pt.cash.ID, pt.cashType, pt.cashAmount, pt.cashDesc, pt.row.Date, investmentTxID,
		pt.keepAccountID == "" || pt.cash.ID != pt.keepAccountID)
}

// keptRate is the FX rate a trade was stored with. An edit that names no
// fx_rate and leaves the trade's date, total and cash leg as they were keeps it
// (including a legacy row's NULL rate), so correcting notes or a typo elsewhere
// never re-prices the trade at today's market rate.
type keptRate struct {
	date       time.Time
	total      int64 // the converted figure: buy cost, sell proceeds, gross dividend
	accountID  string
	cashAmount int64
	rate       *fx.Rate
	base       *int64
}

// priceTrade resolves the trade's FX rate and base amount. cashAmount is the
// amount moved on fundingAccount (nil = no cash leg) in that account's currency
// and cashNative the part of the trade it settles in the security's currency;
// total is the figure that is converted to base.
func (s *investmentService) priceTrade(
	userID, secCcy string, date time.Time, explicit *fx.Rate,
	cashAccount *models.Account, cashAmount, cashNative, total int64,
) (fx.Rate, int64, error) {
	base, err := userBaseCurrency(s.db, userID)
	if err != nil {
		return 0, 0, err
	}
	var leg *tradeCashLeg
	if cashAccount != nil {
		leg = &tradeCashLeg{currency: normalizeCode(cashAccount.Currency), amount: cashAmount, native: cashNative}
	}
	rate, err := s.tradeRate(base, secCcy, date, explicit, leg)
	if err != nil {
		return 0, 0, err
	}
	baseAmount, err := baseAmountFor(total, rate)
	if err != nil {
		return 0, 0, err
	}
	return rate, baseAmount, nil
}

// accountIDOrEmpty returns the cash account the edited trade used, or "" when
// there is no edit (nil) or it had no cash leg.
func (k *keptRate) accountIDOrEmpty() string {
	if k == nil {
		return ""
	}
	return k.accountID
}

// priceOrKeep resolves the trade's FX rate and base amount like priceTrade,
// except that it returns keep's stored figures when the trade is unchanged and
// no explicit rate is given.
func (s *investmentService) priceOrKeep(
	userID, secCcy string, date time.Time, explicit *fx.Rate,
	cashAccount *models.Account, cashAmount, cashNative, total int64, keep *keptRate,
) (*fx.Rate, *int64, error) {
	if explicit == nil && keep != nil {
		accountID := ""
		if cashAccount != nil {
			accountID = cashAccount.ID
		}
		if keep.date.Equal(date) && keep.total == total && keep.accountID == accountID && keep.cashAmount == cashAmount {
			return keep.rate, keep.base, nil
		}
	}
	rate, base, err := s.priceTrade(userID, secCcy, date, explicit, cashAccount, cashAmount, cashNative, total)
	if err != nil {
		return nil, nil, err
	}
	return &rate, &base, nil
}

// prepareBuy validates and prices a buy of investment (Security loaded).
func (s *investmentService) prepareBuy(userID string, investment *models.Investment, p BuyParams, keep *keptRate) (*preparedTrade, error) {
	secCcy := normalizeCode(investment.Security.Currency)
	explicit, err := parseOptionalRate(p.FXRate)
	if err != nil {
		return nil, err
	}
	gross, err := lineAmount(p.Quantity, p.PricePerUnit)
	if err != nil {
		return nil, err
	}
	totalAmount := gross + p.Fee

	fundingAccount, cashAmount, err := s.resolveCashLeg(userID, cashLegRequest{
		accountID: p.FundingAccountID, amount: p.FundingAmount, field: "funding_amount", native: totalAmount, keepAccountID: keep.accountIDOrEmpty(),
	}, secCcy, p.Date)
	if err != nil {
		return nil, err
	}
	rate, baseAmount, err := s.priceOrKeep(userID, secCcy, p.Date, explicit, fundingAccount, cashAmount, totalAmount, totalAmount, keep)
	if err != nil {
		return nil, err
	}
	return &preparedTrade{
		row: models.InvestmentTransaction{
			Type:         models.InvestmentTransactionBuy,
			Date:         p.Date,
			Quantity:     p.Quantity,
			PricePerUnit: p.PricePerUnit,
			TotalAmount:  totalAmount,
			Fee:          p.Fee,
			Notes:        p.Notes,
			FXRateToBase: rate,
			BaseAmount:   baseAmount,
		},
		cash:          fundingAccount,
		cashAmount:    cashAmount,
		cashType:      models.TransactionTypeExpense,
		cashDesc:      fmt.Sprintf("Investment purchase: %g %s", p.Quantity, investment.Security.Name),
		keepAccountID: keep.accountIDOrEmpty(),
	}, nil
}

// prepareSell validates and prices a sell of investment (Security loaded). The
// position is checked by the caller, against the state the sell applies to.
func (s *investmentService) prepareSell(userID string, investment *models.Investment, p SellParams, keep *keptRate) (*preparedTrade, error) {
	secCcy := normalizeCode(investment.Security.Currency)
	explicit, err := parseOptionalRate(p.FXRate)
	if err != nil {
		return nil, err
	}
	gross, err := lineAmount(p.Quantity, p.PricePerUnit)
	if err != nil {
		return nil, err
	}
	totalAmount := gross - p.Fee

	depositAccount, cashAmount, err := s.resolveCashLeg(userID, cashLegRequest{
		accountID: p.DepositAccountID, amount: p.DepositAmount, field: "deposit_amount", native: totalAmount, keepAccountID: keep.accountIDOrEmpty(),
	}, secCcy, p.Date)
	if err != nil {
		return nil, err
	}
	rate, baseAmount, err := s.priceOrKeep(userID, secCcy, p.Date, explicit, depositAccount, cashAmount, totalAmount, totalAmount, keep)
	if err != nil {
		return nil, err
	}
	return &preparedTrade{
		row: models.InvestmentTransaction{
			Type:         models.InvestmentTransactionSell,
			Date:         p.Date,
			Quantity:     p.Quantity,
			PricePerUnit: p.PricePerUnit,
			TotalAmount:  totalAmount,
			Fee:          p.Fee,
			Notes:        p.Notes,
			FXRateToBase: rate,
			BaseAmount:   baseAmount,
		},
		cash:          depositAccount,
		cashAmount:    cashAmount,
		cashType:      models.TransactionTypeIncome,
		cashDesc:      fmt.Sprintf("Sale proceeds: %g %s", p.Quantity, investment.Security.Name),
		keepAccountID: keep.accountIDOrEmpty(),
	}, nil
}

// prepareDividend validates and prices a dividend of investment (Security
// loaded). The cash leg settles the NET dividend; the gross is what gets
// converted to base.
func (s *investmentService) prepareDividend(userID string, investment *models.Investment, p DividendParams, keep *keptRate) (*preparedTrade, error) {
	secCcy := normalizeCode(investment.Security.Currency)
	if p.Fee < 0 {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "fee must not be negative")
	}
	if p.Fee > p.Amount {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "fee must not exceed the dividend amount")
	}
	explicit, err := parseOptionalRate(p.FXRate)
	if err != nil {
		return nil, err
	}
	net := p.Amount - p.Fee

	depositAccount, cashAmount, err := s.resolveCashLeg(userID, cashLegRequest{
		accountID: p.DepositAccountID, amount: p.DepositAmount, field: "deposit_amount", native: net, keepAccountID: keep.accountIDOrEmpty(),
	}, secCcy, p.Date)
	if err != nil {
		return nil, err
	}
	if depositAccount != nil && net <= 0 {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "the net dividend must be positive to deposit it")
	}
	rate, baseAmount, err := s.priceOrKeep(userID, secCcy, p.Date, explicit, depositAccount, cashAmount, net, p.Amount, keep)
	if err != nil {
		return nil, err
	}
	return &preparedTrade{
		row: models.InvestmentTransaction{
			Type:         models.InvestmentTransactionDividend,
			Date:         p.Date,
			TotalAmount:  p.Amount,
			Fee:          p.Fee,
			DividendType: p.DividendType,
			Notes:        p.Notes,
			FXRateToBase: rate,
			BaseAmount:   baseAmount,
		},
		cash:          depositAccount,
		cashAmount:    cashAmount,
		cashType:      models.TransactionTypeIncome,
		cashDesc:      fmt.Sprintf("Dividend: %s", investment.Security.Name),
		keepAccountID: keep.accountIDOrEmpty(),
	}, nil
}

// recordPrepared writes a prepared buy or sell incrementally: it applies the
// trade to the locked holding, stores the row (with the realized figures of a
// sell), updates the holding and writes the linked cash leg.
func (s *investmentService) recordPrepared(userID, investmentID string, pt *preparedTrade) (*models.InvestmentTransaction, error) {
	invTx := pt.row
	invTx.InvestmentID = investmentID
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Re-read (and for a sell re-validate) the holding under a row lock so
		// concurrent writes serialize and two sells can never oversell it.
		locked, txErr := lockInvestment(tx, investmentID)
		if txErr != nil {
			return txErr
		}
		st := stateOf(locked)
		res, txErr := st.apply(eventOf(&invTx))
		if txErr != nil {
			return txErr
		}
		invTx.RealizedGainLoss = res.RealizedGainLoss
		invTx.RealizedGainLossBase = res.RealizedGainLossBase

		if txErr := tx.Create(&invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}
		if txErr := tx.Model(&models.Investment{}).Where("id = ?", investmentID).Updates(st.columns()).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}
		// Move the cash (row-locked, relative update; plan 021, D10).
		return s.writeCashLeg(tx, userID, pt, invTx.ID)
	})
	if err != nil {
		return nil, err
	}
	return &invTx, nil
}

// RecordBuy records a buy transaction and updates the investment holding. The
// price and fee are in the security's currency. If p.FundingAccountID is set the
// cost is deducted from that cash account: in the account's own currency, which
// must be stated in p.FundingAmount when it differs from the security's.
func (s *investmentService) RecordBuy(userID, investmentID string, p BuyParams) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}
	pt, err := s.prepareBuy(userID, investment, p, nil)
	if err != nil {
		return nil, err
	}
	invTx, err := s.recordPrepared(userID, investmentID, pt)
	if err != nil {
		return nil, err
	}
	setCashLeg(invTx, pt)
	invTx.Currency = normalizeCode(investment.Security.Currency)
	return invTx, nil
}

// RecordSell records a sell transaction and adjusts the investment holding
// proportionally, in the security's currency and in base. If p.DepositAccountID
// is set the net proceeds are credited to that cash account: in the account's own
// currency, which must be stated in p.DepositAmount when it differs from the
// security's.
func (s *investmentService) RecordSell(userID, investmentID string, p SellParams) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}
	if p.Quantity > investment.Quantity+quantityEpsilon {
		return nil, apperrors.ErrInsufficientShares
	}
	pt, err := s.prepareSell(userID, investment, p, nil)
	if err != nil {
		return nil, err
	}
	invTx, err := s.recordPrepared(userID, investmentID, pt)
	if err != nil {
		return nil, err
	}
	setCashLeg(invTx, pt)
	invTx.Currency = normalizeCode(investment.Security.Currency)
	return invTx, nil
}

// RecordDividend records a dividend without changing quantity, cost basis or
// realized P&L. p.Amount is the gross dividend in the security's currency and
// p.Fee the withholding tax and fees. With p.DepositAccountID the NET amount
// (gross − fee) is credited to that cash account, in the account's own currency,
// which must be stated in p.DepositAmount when it differs from the security's.
func (s *investmentService) RecordDividend(userID, investmentID string, p DividendParams) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}
	pt, err := s.prepareDividend(userID, investment, p, nil)
	if err != nil {
		return nil, err
	}
	invTx := pt.row
	invTx.InvestmentID = investmentID
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if txErr := tx.Create(&invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}
		return s.writeCashLeg(tx, userID, pt, invTx.ID)
	})
	if err != nil {
		return nil, err
	}
	setCashLeg(&invTx, pt)
	invTx.Currency = normalizeCode(investment.Security.Currency)
	return &invTx, nil
}

// setCashLeg fills the response-only cash leg fields from a prepared trade.
func setCashLeg(invTx *models.InvestmentTransaction, pt *preparedTrade) {
	if pt.cash == nil {
		invTx.CashAccountID, invTx.CashAmount = nil, nil
		return
	}
	id, amount := pt.cash.ID, pt.cashAmount
	invTx.CashAccountID, invTx.CashAmount = &id, &amount
}

// RecordSplit records a stock split and multiplies the investment quantity.
func (s *investmentService) RecordSplit(
	userID, investmentID string,
	date time.Time,
	splitRatio float64,
	notes string,
) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}

	var invTx models.InvestmentTransaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Re-read the holding under a row lock so the split multiplies the
		// current quantity, not a stale pre-transaction read.
		locked, txErr := lockInvestment(tx, investmentID)
		if txErr != nil {
			return txErr
		}
		st := stateOf(locked)
		if _, txErr := st.apply(tradeEvent{Type: models.InvestmentTransactionSplit, SplitRatio: splitRatio}); txErr != nil {
			return txErr
		}

		invTx = models.InvestmentTransaction{
			InvestmentID: investmentID,
			Type:         models.InvestmentTransactionSplit,
			Date:         date,
			Quantity:     locked.Quantity,
			SplitRatio:   splitRatio,
			Notes:        notes,
		}
		if txErr := tx.Create(&invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		// Multiply quantity by split ratio; cost bases stay the same
		if txErr := tx.Model(&models.Investment{}).Where("id = ?", investmentID).Updates(st.columns()).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	invTx.Currency = normalizeCode(investment.Security.Currency)
	return &invTx, nil
}

// GetInvestmentTransactions returns a paginated list of transactions for an investment.
func (s *investmentService) GetInvestmentTransactions(userID, investmentID string, page pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error) {
	// Verify investment exists and user owns it
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}

	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.InvestmentTransaction{}).Where("investment_id = ?", investmentID)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var transactions []models.InvestmentTransaction
	if err := base.Order("date DESC, created_at DESC, id DESC").Scopes(pagination.Paginate(page)).Find(&transactions).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	currency := normalizeCode(investment.Security.Currency)
	for i := range transactions {
		transactions[i].Currency = currency
	}
	if err := attachCashLegs(s.db, userID, investment, transactions); err != nil {
		return nil, err
	}

	result := pagination.NewPageResponse(transactions, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// lockInvestment re-reads an investment row with a row lock (SELECT ... FOR
// UPDATE on Postgres) inside an open DB transaction. Ownership must already
// have been checked by the caller (GetInvestmentByID).
func lockInvestment(tx *gorm.DB, investmentID string) (*models.Investment, error) {
	var inv models.Investment
	if err := lockUpdate(tx).Where("id = ?", investmentID).First(&inv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrInvestmentNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &inv, nil
}
