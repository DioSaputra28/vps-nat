package users

import (
	"context"
	"errors"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

const (
	defaultPage  = 1
	defaultLimit = 10
	maxLimit     = 100
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPagination = errors.New("invalid pagination")
)

type Service struct {
	repo *Repository
}

type ListInput struct {
	Page   int
	Limit  int
	Search string
}

type ListResult struct {
	Items      []model.User
	Page       int
	Limit      int
	TotalItems int64
	TotalPages int
	Search     string
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, input ListInput) (*ListResult, error) {
	page := input.Page
	limit := input.Limit

	if page == 0 {
		page = defaultPage
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if page <= 0 || limit <= 0 || limit > maxLimit {
		return nil, ErrInvalidPagination
	}

	search := strings.TrimSpace(input.Search)
	items, totalItems, err := s.repo.FindAll(ctx, ListParams{
		Page:   page,
		Limit:  limit,
		Search: search,
	})
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(limit) - 1) / int64(limit))
	}

	return &ListResult{
		Items:      items,
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
		Search:     search,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*model.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}

		return nil, err
	}

	return user, nil
}
