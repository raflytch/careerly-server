package handler

import (
	"errors"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/middleware"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/response"
	"github.com/raflytch/careerly-server/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ATSCheckHandler struct {
	atsCheckService domain.ATSCheckService
	quotaService    domain.QuotaService
	fileValidator   *validator.FileValidator
}

func NewATSCheckHandler(atsCheckService domain.ATSCheckService, quotaService domain.QuotaService) *ATSCheckHandler {
	return &ATSCheckHandler{
		atsCheckService: atsCheckService,
		quotaService:    quotaService,
		fileValidator: validator.NewFileValidator(
			validator.WithMaxSize(validator.MaxSize5MB),
			validator.WithAllowedTypes([]string{".pdf"}),
		),
	}
}

func (h *ATSCheckHandler) Analyze(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "pdf file is required, use form field 'file'")
	}

	if err := h.fileValidator.Validate(file); err != nil {
		return response.BadRequest(c, err.Error())
	}

	result, err := h.atsCheckService.AnalyzeFromFile(c.UserContext(), user.ID, file)
	if err != nil {
		if errors.Is(err, service.ErrAIClientUnavailable) {
			return response.InternalError(c, "ai service is unavailable, cannot analyze pdf")
		}
		if errors.Is(err, service.ErrNoActiveSubscription) {
			return response.Forbidden(c, "no active subscription found")
		}
		if errors.Is(err, service.ErrQuotaExceeded) {
			return response.Forbidden(c, "ats check quota exceeded for this month")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusCreated, "ats analysis completed", result)
}

func (h *ATSCheckHandler) GetByID(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid ats check id")
	}

	check, err := h.atsCheckService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrATSCheckNotFound) {
			return response.NotFound(c, "ats check not found")
		}
		if errors.Is(err, service.ErrATSCheckUnauthorized) {
			return response.Forbidden(c, "unauthorized access to ats check")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "ats check retrieved", check)
}

func (h *ATSCheckHandler) GetMyATSChecks(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	result, err := h.atsCheckService.GetByUserID(c.UserContext(), user.ID, page, limit)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "ats checks retrieved", result)
}

func (h *ATSCheckHandler) Delete(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid ats check id")
	}

	if err := h.atsCheckService.Delete(c.UserContext(), user.ID, id); err != nil {
		if errors.Is(err, service.ErrATSCheckNotFound) {
			return response.NotFound(c, "ats check not found")
		}
		if errors.Is(err, service.ErrATSCheckUnauthorized) {
			return response.Forbidden(c, "unauthorized access to ats check")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "ats check deleted", nil)
}
