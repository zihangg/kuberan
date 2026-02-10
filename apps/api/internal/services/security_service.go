package services

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// securityService handles security-related business logic.
type securityService struct {
	db *gorm.DB
}

// NewSecurityService creates a new SecurityServicer.
func NewSecurityService(db *gorm.DB) SecurityServicer {
	return &securityService{db: db}
}

// CreateSecurity creates a new security record.
func (s *securityService) CreateSecurity(
	symbol, name string,
	assetType models.AssetType,
	currency, exchange string,
	extraFields map[string]interface{},
) (*models.Security, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "Symbol is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "Name is required")
	}
	if currency == "" {
		currency = "USD"
	}

	security := &models.Security{
		Symbol:    symbol,
		Name:      name,
		AssetType: assetType,
		Currency:  currency,
		Exchange:  exchange,
	}

	applySecurityExtraFields(security, extraFields)

	if err := s.db.Create(security).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, apperrors.ErrDuplicateSecurity
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	return security, nil
}

// GetSecurityByID returns a security by its ID.
func (s *securityService) GetSecurityByID(id uint) (*models.Security, error) {
	var security models.Security
	if err := s.db.First(&security, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSecurityNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &security, nil
}

// ListSecurities returns a paginated list of securities ordered by symbol.
func (s *securityService) ListSecurities(page pagination.PageRequest) (*pagination.PageResponse[models.Security], error) {
	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.Security{})
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var securities []models.Security
	if err := base.Order("symbol ASC").Scopes(pagination.Paginate(page)).Find(&securities).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(securities, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// RecordPrices bulk-inserts price entries, skipping duplicates.
func (s *securityService) RecordPrices(prices []SecurityPriceInput) (int, error) {
	if len(prices) == 0 {
		return 0, apperrors.WithMessage(apperrors.ErrInvalidInput, "Prices array is empty")
	}

	count := 0
	for _, p := range prices {
		sp := models.SecurityPrice{
			SecurityID: p.SecurityID,
			Price:      p.Price,
			RecordedAt: p.RecordedAt,
		}
		result := s.db.Where("security_id = ? AND recorded_at = ?", sp.SecurityID, sp.RecordedAt).
			FirstOrCreate(&sp)
		if result.Error != nil {
			return count, apperrors.Wrap(apperrors.ErrInternalServer, result.Error)
		}
		if result.RowsAffected > 0 {
			count++
		}
	}

	return count, nil
}

// GetPriceHistory returns paginated price history for a security within a date range.
func (s *securityService) GetPriceHistory(
	securityID uint,
	from, to time.Time,
	page pagination.PageRequest,
) (*pagination.PageResponse[models.SecurityPrice], error) {
	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.SecurityPrice{}).
		Where("security_id = ? AND recorded_at >= ? AND recorded_at <= ?", securityID, from, to)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var prices []models.SecurityPrice
	if err := base.Order("recorded_at DESC").Scopes(pagination.Paginate(page)).Find(&prices).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(prices, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// applySecurityExtraFields sets asset-type-specific fields on a security from a map.
func applySecurityExtraFields(sec *models.Security, fields map[string]interface{}) {
	if fields == nil {
		return
	}
	if v, ok := fields["maturity_date"].(*time.Time); ok {
		sec.MaturityDate = v
	}
	if v, ok := fields["yield_to_maturity"].(float64); ok {
		sec.YieldToMaturity = v
	}
	if v, ok := fields["coupon_rate"].(float64); ok {
		sec.CouponRate = v
	}
	if v, ok := fields["network"].(string); ok {
		sec.Network = v
	}
	if v, ok := fields["property_type"].(string); ok {
		sec.PropertyType = v
	}
}

// isUniqueConstraintError checks if a GORM error is a unique constraint violation.
func isUniqueConstraintError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || // SQLite
		strings.Contains(msg, "duplicate key value violates unique constraint") // PostgreSQL
}
