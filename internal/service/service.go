// Package service holds the business logic ported from the Java *ServiceImpl
// classes. The Java code used Spring DI with several interfaces that call each
// other (Content<->Meta<->Relationship, Comment->Content). To keep that graph
// without Go import cycles, all logic lives on one aggregate Service value.
package service

import (
	"errors"
	"sync"
	"sync/atomic"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/model"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"strconv"
)

// TipError is the Go analogue of the Java TipException: a user-facing message
// that handlers surface directly rather than logging as an internal error.
type TipError struct{ Msg string }

func (e *TipError) Error() string { return e.Msg }

// Tip creates a TipError.
func Tip(msg string) error { return &TipError{Msg: msg} }

// txLike aliases *gorm.DB so transaction closures read clearly.
type txLike = *gorm.DB

// AsTip returns the message if err is a TipError, and whether it was one.
func AsTip(err error) (string, bool) {
	var t *TipError
	if errors.As(err, &t) {
		return t.Msg, true
	}
	return "", false
}

// Service aggregates every domain service. Methods are split across files
// (content.go, comment.go, meta.go, ...) but share this receiver.
type Service struct {
	db    *gorm.DB
	cache *cache.Cache
	cfg   *config.Config
	sf    singleflight.Group // collapses duplicate expensive reads under load

	contentListVersion atomic.Uint64
	commentVersions    sync.Map // cid -> *atomic.Uint64
}

// New constructs the aggregate service.
func New(db *gorm.DB, c *cache.Cache, cfg *config.Config) *Service {
	return &Service{db: db, cache: c, cfg: cfg}
}

// DB exposes the underlying gorm handle (used for health checks / stats).
func (s *Service) DB() *gorm.DB { return s.db }

// Cache exposes the shared cache (used by handlers for CSRF tokens etc).
func (s *Service) Cache() *cache.Cache { return s.cache }

// gormExprAdd builds a "column = column + delta" expression for atomic
// increments at the SQL level (avoids read-modify-write races).
func gormExprAdd(column string, delta int) interface{} {
	return gorm.Expr(column+" + ?", delta)
}

func (s *Service) invalidateContent(content *model.Content) {
	if content == nil {
		return
	}
	s.cache.Del("content:" + strconv.Itoa(content.Cid))
	if content.Slug != "" {
		s.cache.Del("content:" + content.Slug)
	}
	s.contentListVersion.Add(1)
}

func (s *Service) commentVersion(cid int) uint64 {
	value, _ := s.commentVersions.LoadOrStore(cid, &atomic.Uint64{})
	return value.(*atomic.Uint64).Load()
}

func (s *Service) invalidateComments(cid int) {
	value, _ := s.commentVersions.LoadOrStore(cid, &atomic.Uint64{})
	value.(*atomic.Uint64).Add(1)
}
