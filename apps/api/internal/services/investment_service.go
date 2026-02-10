package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// investmentService handles investment-related business logic.
type investmentService struct {
	db             *gorm.DB
	accountService AccountServicer
}

// NewInvestmentService creates a new InvestmentServicer.
func NewInvestmentService(db *gorm.DB, accountService AccountServicer) InvestmentServicer {
	return &investmentService{db: db, accountService: accountService}
}

// AddInvestment adds a new investment holding to an investment account.
func (s *investmentService) AddInvestment(
	userID, accountID, securityID uint,
	quantity float64,
	purchasePrice int64,
	walletAddress string,
	date *time.Time,
	fee int64,
	notes string,
) (*models.Investment, error) {
	// Verify account exists, belongs to user, and is an investment account
	account, err := s.accountService.GetAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}
	if account.Type != models.AccountTypeInvestment {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "Account is not an investment account")
	}

	// Verify security exists
	var security models.Security
	if err := s.db.First(&security, securityID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSecurityNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Apply defaults for optional fields
	txDate := time.Now()
	if date != nil {
		txDate = *date
	}
	txNotes := "Initial purchase"
	if notes != "" {
		txNotes = notes
	}

	costBasis := int64(quantity*float64(purchasePrice)) + fee

	investment := &models.Investment{
		AccountID:     accountID,
		SecurityID:    securityID,
		Quantity:      quantity,
		CostBasis:     costBasis,
		CurrentPrice:  purchasePrice,
		LastUpdated:   time.Now(),
		WalletAddress: walletAddress,
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
			Quantity:     quantity,
			PricePerUnit: purchasePrice,
			TotalAmount:  costBasis,
			Fee:          fee,
			Notes:        txNotes,
		}
		if txErr := tx.Create(invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	investment.Security = security
	return investment, nil
}

// GetAccountInvestments returns a paginated list of investments for an account.
func (s *investmentService) GetAccountInvestments(userID, accountID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
	// Verify account exists and belongs to user
	if _, err := s.accountService.GetAccountByID(userID, accountID); err != nil {
		return nil, err
	}

	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.Investment{}).Where("account_id = ?", accountID)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var investments []models.Investment
	if err := s.db.Preload("Security").Where("account_id = ?", accountID).
		Scopes(pagination.Paginate(page)).Find(&investments).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(investments, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// GetInvestmentByID returns an investment if the parent account belongs to the user.
func (s *investmentService) GetInvestmentByID(userID, investmentID uint) (*models.Investment, error) {
	var investment models.Investment
	if err := s.db.Preload("Account").Preload("Security").First(&investment, investmentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrInvestmentNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Verify account belongs to user
	if investment.Account.UserID != userID {
		return nil, apperrors.ErrInvestmentNotFound
	}

	return &investment, nil
}

// UpdateInvestmentPrice updates the current market price of an investment.
func (s *investmentService) UpdateInvestmentPrice(userID, investmentID uint, currentPrice int64) (*models.Investment, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.db.Model(investment).Updates(map[string]interface{}{
		"current_price": currentPrice,
		"last_updated":  now,
	}).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	investment.CurrentPrice = currentPrice
	investment.LastUpdated = now
	return investment, nil
}

// GetPortfolio returns an aggregated portfolio summary across all investment accounts.
func (s *investmentService) GetPortfolio(userID uint) (*PortfolioSummary, error) {
	// Get all investment accounts for the user
	var accounts []models.Account
	if err := s.db.Where("user_id = ? AND type = ? AND is_active = ?", userID, models.AccountTypeInvestment, true).
		Find(&accounts).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	accountIDs := make([]uint, len(accounts))
	for i := range accounts {
		accountIDs[i] = accounts[i].ID
	}

	summary := &PortfolioSummary{
		HoldingsByType: make(map[models.AssetType]TypeSummary),
	}

	if len(accountIDs) == 0 {
		return summary, nil
	}

	// Get all investments across those accounts with Security preloaded
	var investments []models.Investment
	if err := s.db.Preload("Security").Where("account_id IN ?", accountIDs).Find(&investments).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	for i := range investments {
		inv := &investments[i]
		value := int64(inv.Quantity * float64(inv.CurrentPrice))
		summary.TotalValue += value
		summary.TotalCostBasis += inv.CostBasis

		ts := summary.HoldingsByType[inv.Security.AssetType]
		ts.Value += value
		ts.Count++
		summary.HoldingsByType[inv.Security.AssetType] = ts
	}

	summary.TotalGainLoss = summary.TotalValue - summary.TotalCostBasis
	if summary.TotalCostBasis > 0 {
		summary.GainLossPct = float64(summary.TotalGainLoss) / float64(summary.TotalCostBasis) * 100
	}

	return summary, nil
}

// RecordBuy records a buy transaction and updates the investment holding.
func (s *investmentService) RecordBuy(
	userID, investmentID uint,
	date time.Time,
	quantity float64,
	pricePerUnit int64,
	fee int64,
	notes string,
) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}

	totalAmount := int64(quantity*float64(pricePerUnit)) + fee

	var invTx models.InvestmentTransaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		invTx = models.InvestmentTransaction{
			InvestmentID: investmentID,
			Type:         models.InvestmentTransactionBuy,
			Date:         date,
			Quantity:     quantity,
			PricePerUnit: pricePerUnit,
			TotalAmount:  totalAmount,
			Fee:          fee,
			Notes:        notes,
		}
		if txErr := tx.Create(&invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		// Update investment: quantity and cost basis increase
		newQuantity := investment.Quantity + quantity
		newCostBasis := investment.CostBasis + totalAmount
		if txErr := tx.Model(investment).Updates(map[string]interface{}{
			"quantity":   newQuantity,
			"cost_basis": newCostBasis,
		}).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &invTx, nil
}

// RecordSell records a sell transaction and adjusts the investment holding proportionally.
func (s *investmentService) RecordSell(
	userID, investmentID uint,
	date time.Time,
	quantity float64,
	pricePerUnit int64,
	fee int64,
	notes string,
) (*models.InvestmentTransaction, error) {
	investment, err := s.GetInvestmentByID(userID, investmentID)
	if err != nil {
		return nil, err
	}

	if quantity > investment.Quantity {
		return nil, apperrors.ErrInsufficientShares
	}

	totalAmount := int64(quantity*float64(pricePerUnit)) - fee

	// Proportional cost basis reduction
	costBasisReduction := int64(float64(investment.CostBasis) * (quantity / investment.Quantity))

	var invTx models.InvestmentTransaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		invTx = models.InvestmentTransaction{
			InvestmentID: investmentID,
			Type:         models.InvestmentTransactionSell,
			Date:         date,
			Quantity:     quantity,
			PricePerUnit: pricePerUnit,
			TotalAmount:  totalAmount,
			Fee:          fee,
			Notes:        notes,
		}
		if txErr := tx.Create(&invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		newQuantity := investment.Quantity - quantity
		newCostBasis := investment.CostBasis - costBasisReduction
		if txErr := tx.Model(investment).Updates(map[string]interface{}{
			"quantity":   newQuantity,
			"cost_basis": newCostBasis,
		}).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &invTx, nil
}

// RecordDividend records a dividend transaction without changing quantity or cost basis.
func (s *investmentService) RecordDividend(
	userID, investmentID uint,
	date time.Time,
	amount int64,
	dividendType, notes string,
) (*models.InvestmentTransaction, error) {
	if _, err := s.GetInvestmentByID(userID, investmentID); err != nil {
		return nil, err
	}

	invTx := &models.InvestmentTransaction{
		InvestmentID: investmentID,
		Type:         models.InvestmentTransactionDividend,
		Date:         date,
		TotalAmount:  amount,
		DividendType: dividendType,
		Notes:        notes,
	}

	if err := s.db.Create(invTx).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	return invTx, nil
}

// RecordSplit records a stock split and multiplies the investment quantity.
func (s *investmentService) RecordSplit(
	userID, investmentID uint,
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
		invTx = models.InvestmentTransaction{
			InvestmentID: investmentID,
			Type:         models.InvestmentTransactionSplit,
			Date:         date,
			Quantity:     investment.Quantity,
			SplitRatio:   splitRatio,
			Notes:        notes,
		}
		if txErr := tx.Create(&invTx).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		// Multiply quantity by split ratio; cost basis stays the same
		newQuantity := investment.Quantity * splitRatio
		if txErr := tx.Model(investment).Update("quantity", newQuantity).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &invTx, nil
}

// GetInvestmentTransactions returns a paginated list of transactions for an investment.
func (s *investmentService) GetInvestmentTransactions(userID, investmentID uint, page pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error) {
	// Verify investment exists and user owns it
	if _, err := s.GetInvestmentByID(userID, investmentID); err != nil {
		return nil, err
	}

	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.InvestmentTransaction{}).Where("investment_id = ?", investmentID)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var transactions []models.InvestmentTransaction
	if err := base.Order("date DESC").Scopes(pagination.Paginate(page)).Find(&transactions).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(transactions, page.Page, page.PageSize, totalItems)
	return &result, nil
}
