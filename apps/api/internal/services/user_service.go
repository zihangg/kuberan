package services

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute

	// refreshRotationGrace is how long a rotated-out refresh token stays
	// usable for its session.
	refreshRotationGrace = 2 * time.Minute
	// maxRefreshSessionsPerUser caps concurrently signed-in devices per user.
	maxRefreshSessionsPerUser = 20
)

// userService handles user-related business logic.
type userService struct {
	db *gorm.DB
}

// NewUserService creates a new UserServicer.
func NewUserService(db *gorm.DB) UserServicer {
	return &userService{db: db}
}

// CreateUser registers a new user
func (s *userService) CreateUser(email, password, firstName, lastName string) (*models.User, error) {
	// Validate input
	if email == "" || password == "" {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "email and password are required")
	}

	// Check if user with email exists
	var count int64
	s.db.Model(&models.User{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil, apperrors.ErrDuplicateEmail
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Create user
	user := &models.User{
		Email:     strings.ToLower(email),
		Password:  string(hashedPassword),
		FirstName: firstName,
		LastName:  lastName,
		IsActive:  true,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (s *userService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("email = ? AND is_active = ?", strings.ToLower(email), true).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *userService) GetUserByID(id string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &user, nil
}

// VerifyPassword checks if the provided password matches the stored hash
func (s *userService) VerifyPassword(user *models.User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

// AttemptLogin authenticates a user by email and password with lockout protection.
// Returns the user on success, or an appropriate AppError on failure.
func (s *userService) AttemptLogin(email, password string) (*models.User, error) {
	user, err := s.GetUserByEmail(email)
	if err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, apperrors.ErrAccountLocked
	}

	// Verify password
	if !s.VerifyPassword(user, password) {
		user.FailedLoginAttempts++
		if user.FailedLoginAttempts >= maxFailedAttempts {
			lockUntil := time.Now().Add(lockoutDuration)
			user.LockedUntil = &lockUntil
		}
		s.saveLoginState(user)
		return nil, apperrors.ErrInvalidCredentials
	}

	// Successful login: reset failed attempts
	now := time.Now()
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	user.LastLoginAt = &now
	s.saveLoginState(user)

	return user, nil
}

// saveLoginState writes only the login-tracking columns. Saving the whole row
// would write back the copy read before the (slow) bcrypt check and undo any
// profile change made in the meantime.
func (s *userService) saveLoginState(user *models.User) {
	s.db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"failed_login_attempts": user.FailedLoginAttempts,
		"locked_until":          user.LockedUntil,
		"last_login_at":         user.LastLoginAt,
	})
}

// CreateRefreshSession starts a new per-device refresh session holding the
// hash of its first refresh token. Expired sessions for the user are pruned,
// and the oldest are dropped beyond maxRefreshSessionsPerUser, so the table
// stays bounded even if clients never log out.
func (s *userService) CreateRefreshSession(userID, tokenHash, userAgent, ipAddress string, expiresAt time.Time) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ? AND expires_at <= ?", userID, time.Now()).
			Delete(&models.RefreshSession{}).Error; err != nil {
			return err
		}

		session := models.RefreshSession{
			UserID:    userID,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			UserAgent: truncateRunes(userAgent, 255),
			IPAddress: truncateRunes(ipAddress, 45),
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}

		var stale []string
		if err := tx.Model(&models.RefreshSession{}).Where("user_id = ?", userID).
			Order("created_at DESC").Offset(maxRefreshSessionsPerUser).Pluck("id", &stale).Error; err != nil {
			return err
		}
		if len(stale) > 0 {
			return tx.Unscoped().Where("id IN ?", stale).Delete(&models.RefreshSession{}).Error
		}
		return nil
	})
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

// RotateRefreshSession swaps oldHash for newHash in the user's session that
// holds it and extends the session to expiresAt. oldHash is accepted if it is
// the session's current token, or the token rotated out less than
// refreshRotationGrace ago (two tabs refreshing at once, or a lost response).
// Each branch is a single conditional UPDATE, so concurrent rotations of the
// same token cannot both win the current-token branch. Returns ErrUnauthorized
// when no live session matches.
func (s *userService) RotateRefreshSession(userID, oldHash, newHash string, expiresAt time.Time) error {
	now := time.Now()

	res := s.db.Model(&models.RefreshSession{}).
		Where("user_id = ? AND token_hash = ? AND expires_at > ?", userID, oldHash, now).
		Updates(map[string]interface{}{
			"previous_token_hash": gorm.Expr("token_hash"),
			"token_hash":          newHash,
			"rotated_at":          now,
			"expires_at":          expiresAt,
		})
	if res.Error != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, res.Error)
	}
	if res.RowsAffected == 1 {
		return nil
	}

	// Grace path: keep previous_token_hash as is so the window is measured
	// from the first rotation and cannot be extended by replaying the token.
	res = s.db.Model(&models.RefreshSession{}).
		Where("user_id = ? AND previous_token_hash = ? AND rotated_at > ? AND expires_at > ?",
			userID, oldHash, now.Add(-refreshRotationGrace), now).
		Updates(map[string]interface{}{
			"token_hash": newHash,
			"expires_at": expiresAt,
		})
	if res.Error != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, res.Error)
	}
	if res.RowsAffected == 1 {
		return nil
	}
	return apperrors.ErrUnauthorized
}

// RevokeRefreshSession ends the session holding tokenHash (logout). Revoking a
// token that matches no session is not an error.
func (s *userService) RevokeRefreshSession(userID, tokenHash string) error {
	if err := s.db.Unscoped().
		Where("user_id = ? AND (token_hash = ? OR previous_token_hash = ?)", userID, tokenHash, tokenHash).
		Delete(&models.RefreshSession{}).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

// ProfileSettingsUpdate lists the profile settings to change. A nil
// HideBalances leaves it as is; DefaultAccountID is only applied when
// SetDefaultAccount is true, and a nil DefaultAccountID then clears it.
type ProfileSettingsUpdate struct {
	HideBalances      *bool
	SetDefaultAccount bool
	DefaultAccountID  *string
}

// UpdateProfileSettings applies the given settings in a single update, so a
// request changing several of them succeeds or fails as a whole.
//
// A default account is the one preselected when the user records a new
// transaction. It must belong to the user, be active, and be a spending
// account (cash, credit card or debt).
func (s *userService) UpdateProfileSettings(userID string, update ProfileSettingsUpdate) error {
	changes := map[string]interface{}{}
	if update.HideBalances != nil {
		changes["hide_balances"] = *update.HideBalances
	}
	if update.SetDefaultAccount {
		if update.DefaultAccountID != nil {
			if err := s.validateDefaultAccount(userID, *update.DefaultAccountID); err != nil {
				return err
			}
		}
		changes["default_account_id"] = update.DefaultAccountID
	}
	if len(changes) == 0 {
		return apperrors.WithMessage(apperrors.ErrInvalidInput, "no profile settings to update")
	}

	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Updates(changes).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

// validateDefaultAccount checks that accountID may be the user's default account.
func (s *userService) validateDefaultAccount(userID, accountID string) error {
	var account models.Account
	err := s.db.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperrors.ErrAccountNotFound
	}
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if !account.IsActive {
		return apperrors.WithMessage(apperrors.ErrInvalidInput, "default account must be active")
	}
	if !account.Type.IsSpending() {
		return apperrors.WithMessage(apperrors.ErrInvalidInput,
			"default account must be a cash, credit card or debt account")
	}
	return nil
}
