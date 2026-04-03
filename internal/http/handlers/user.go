package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/DioSaputra28/vps-nat/internal/users"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service *users.Service
}

type walletSummaryResponse struct {
	ID        string `json:"id"`
	Balance   int64  `json:"balance"`
	CreatedAt any    `json:"created_at"`
	UpdatedAt any    `json:"updated_at"`
}

type userResponse struct {
	ID               string                 `json:"id"`
	TelegramID       int64                  `json:"telegram_id"`
	TelegramUsername *string                `json:"telegram_username,omitempty"`
	DisplayName      string                 `json:"display_name"`
	Status           string                 `json:"status"`
	Wallet           *walletSummaryResponse `json:"wallet,omitempty"`
	CreatedAt        any                    `json:"created_at"`
	UpdatedAt        any                    `json:"updated_at"`
}

func NewUserHandler(service *users.Service) UserHandler {
	return UserHandler{service: service}
}

func (h UserHandler) List(c *gin.Context) {
	page, err := parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "page must be a positive integer", "bad_request", nil)
		return
	}

	limit, err := parsePositiveInt(c.Query("limit"), 10)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "limit must be a positive integer", "bad_request", nil)
		return
	}

	result, err := h.service.List(c.Request.Context(), users.ListInput{
		Page:   page,
		Limit:  limit,
		Search: c.Query("search"),
	})
	if err != nil {
		handleUserError(c, err, "failed to list users")
		return
	}

	response.Success(c, http.StatusOK, "users fetched successfully", gin.H{
		"items": toUserResponses(result.Items),
		"meta": gin.H{
			"page":        result.Page,
			"limit":       result.Limit,
			"total_items": result.TotalItems,
			"total_pages": result.TotalPages,
			"search":      result.Search,
		},
	})
}

func (h UserHandler) GetByID(c *gin.Context) {
	user, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleUserError(c, err, "failed to get user")
		return
	}

	response.Success(c, http.StatusOK, "user fetched successfully", toUserResponse(user))
}

func handleUserError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, users.ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, "user not found", "not_found", nil)
	case errors.Is(err, users.ErrInvalidPagination):
		response.Fail(c, http.StatusBadRequest, err.Error(), "bad_request", nil)
	default:
		response.Fail(c, http.StatusInternalServerError, fallback, "internal_server_error", nil)
	}
}

func toUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:               user.ID,
		TelegramID:       user.TelegramID,
		TelegramUsername: user.TelegramUsername,
		DisplayName:      user.DisplayName,
		Status:           user.Status,
		Wallet:           toWalletSummaryResponse(user.Wallet),
		CreatedAt:        user.CreatedAt,
		UpdatedAt:        user.UpdatedAt,
	}
}

func toUserResponses(items []model.User) []userResponse {
	result := make([]userResponse, 0, len(items))
	for i := range items {
		result = append(result, toUserResponse(&items[i]))
	}

	return result
}

func toWalletSummaryResponse(wallet *model.Wallet) *walletSummaryResponse {
	if wallet == nil {
		return nil
	}

	return &walletSummaryResponse{
		ID:        wallet.ID,
		Balance:   wallet.Balance,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}
}

func parsePositiveInt(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid positive integer")
	}

	return value, nil
}
