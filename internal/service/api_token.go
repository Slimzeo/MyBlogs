package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"myblog/internal/model"
)

const apiTokenVersion = "mb_v1"

var (
	ErrInvalidAPIToken   = errors.New("invalid API token")
	ErrAPITokenForbidden = errors.New("API token lacks the required scope")
)

// CreateAPIToken creates an expiring, scoped token. The plaintext value is
// returned once and only its digest is stored in the database.
func (s *Service) CreateAPIToken(name, scope string, authorID, validDays int) (*model.APIToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return nil, "", Tip("密钥名称应为1-80个字符")
	}
	if scope != model.ScopeArticleImport {
		return nil, "", Tip("密钥权限不合法")
	}
	if validDays < 1 || validDays > 365 {
		return nil, "", Tip("有效期应为1-365天")
	}
	if s.QueryUserByID(authorID) == nil {
		return nil, "", Tip("管理员不存在")
	}

	tokenID, err := randomTokenPart(12)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomTokenPart(32)
	if err != nil {
		return nil, "", err
	}
	plaintext := apiTokenVersion + "." + tokenID + "." + secret
	record := &model.APIToken{
		TokenID:    tokenID,
		Name:       name,
		SecretHash: hashAPITokenSecret(secret),
		Scope:      scope,
		AuthorID:   authorID,
		Created:    int(time.Now().Unix()),
		Expires:    int(time.Now().Add(time.Duration(validDays) * 24 * time.Hour).Unix()),
	}
	if err := s.db.Create(record).Error; err != nil {
		return nil, "", err
	}
	return record, plaintext, nil
}

func (s *Service) APITokens(authorID int) ([]model.APIToken, error) {
	var tokens []model.APIToken
	err := s.db.Where("author_id = ?", authorID).Order("id desc").Find(&tokens).Error
	return tokens, err
}

func (s *Service) RevokeAPIToken(id, authorID int) error {
	if id <= 0 || authorID <= 0 {
		return Tip("密钥不存在")
	}
	result := s.db.Model(&model.APIToken{}).
		Where("id = ? AND author_id = ? AND revoked = 0", id, authorID).
		Update("revoked", int(time.Now().Unix()))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return Tip("密钥不存在或已撤销")
	}
	return nil
}

// AuthenticateAPIToken validates a bearer token and its required scope.
func (s *Service) AuthenticateAPIToken(plaintext, requiredScope string) (*model.APIToken, error) {
	parts := strings.Split(plaintext, ".")
	if len(parts) != 3 || parts[0] != apiTokenVersion || parts[1] == "" || parts[2] == "" {
		return nil, ErrInvalidAPIToken
	}
	var token model.APIToken
	if err := s.db.Where("token_id = ?", parts[1]).First(&token).Error; err != nil {
		return nil, ErrInvalidAPIToken
	}
	expected, err := hex.DecodeString(token.SecretHash)
	if err != nil {
		return nil, ErrInvalidAPIToken
	}
	actual := sha256.Sum256([]byte(parts[2]))
	if !hmac.Equal(expected, actual[:]) || token.Revoked != 0 || token.Expires <= int(time.Now().Unix()) {
		return nil, ErrInvalidAPIToken
	}
	if requiredScope != "" && !apiTokenHasScope(token.Scope, requiredScope) {
		return nil, ErrAPITokenForbidden
	}
	if s.QueryUserByID(token.AuthorID) == nil {
		return nil, ErrInvalidAPIToken
	}
	if token.LastUsed < int(time.Now().Add(-5*time.Minute).Unix()) {
		now := int(time.Now().Unix())
		_ = s.db.Model(&model.APIToken{}).Where("id = ?", token.ID).Update("last_used", now).Error
		token.LastUsed = now
	}
	return &token, nil
}

func randomTokenPart(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashAPITokenSecret(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(digest[:])
}

func apiTokenHasScope(scopes, required string) bool {
	for _, scope := range strings.Split(scopes, ",") {
		if strings.TrimSpace(scope) == required {
			return true
		}
	}
	return false
}
