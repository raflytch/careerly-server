package handler

import (
	"errors"
	"fmt"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/middleware"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ResumeHandler struct {
	resumeService domain.ResumeService
	quotaService  domain.QuotaService
}

func NewResumeHandler(resumeService domain.ResumeService, quotaService domain.QuotaService) *ResumeHandler {
	return &ResumeHandler{
		resumeService: resumeService,
		quotaService:  quotaService,
	}
}

func (h *ResumeHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	var req domain.CreateResumeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if err := validateResumeRequest(&req); err != nil {
		return response.BadRequest(c, err.Error())
	}

	result, err := h.resumeService.Create(c.UserContext(), user.ID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveSubscription) {
			return response.Forbidden(c, "no active subscription found")
		}
		if errors.Is(err, service.ErrQuotaExceeded) {
			return response.Forbidden(c, "resume quota exceeded for this month")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusCreated, "resume created", result)
}

func (h *ResumeHandler) GetByID(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid resume id")
	}

	resume, err := h.resumeService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrResumeNotFound) {
			return response.NotFound(c, "resume not found")
		}
		if errors.Is(err, service.ErrUnauthorized) {
			return response.Forbidden(c, "unauthorized access to resume")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "resume retrieved", resume)
}

func (h *ResumeHandler) GetMyResumes(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	result, err := h.resumeService.GetByUserID(c.UserContext(), user.ID, page, limit)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "resumes retrieved", result)
}

func (h *ResumeHandler) Update(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid resume id")
	}

	var req domain.UpdateResumeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	result, err := h.resumeService.Update(c.UserContext(), user.ID, id, &req)
	if err != nil {
		if errors.Is(err, service.ErrResumeNotFound) {
			return response.NotFound(c, "resume not found")
		}
		if errors.Is(err, service.ErrUnauthorized) {
			return response.Forbidden(c, "unauthorized access to resume")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "resume updated", result)
}

func (h *ResumeHandler) Delete(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid resume id")
	}

	if err := h.resumeService.Delete(c.UserContext(), user.ID, id); err != nil {
		if errors.Is(err, service.ErrResumeNotFound) {
			return response.NotFound(c, "resume not found")
		}
		if errors.Is(err, service.ErrUnauthorized) {
			return response.Forbidden(c, "unauthorized access to resume")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "resume deleted", nil)
}

func (h *ResumeHandler) DownloadPDF(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid resume id")
	}

	pdfBytes, err := h.resumeService.GeneratePDF(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrResumeNotFound) {
			return response.NotFound(c, "resume not found")
		}
		if errors.Is(err, service.ErrUnauthorized) {
			return response.Forbidden(c, "unauthorized access to resume")
		}
		return response.InternalError(c, err.Error())
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=resume_%s.pdf", id.String()))
	return c.Send(pdfBytes)
}

func (h *ResumeHandler) GetQuota(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	quota, err := h.quotaService.GetUserQuota(c.UserContext(), user.ID)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveSubscription) {
			return response.Forbidden(c, "no active subscription found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "quota retrieved", quota)
}

var resumeValidator = validator.New()

func validateResumeRequest(req interface{}) error {
	if err := resumeValidator.Struct(req); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, e := range validationErrors {
				field := e.Field()
				switch e.Tag() {
				case "required":
					return fmt.Errorf("%s is required", field)
				case "min":
					return fmt.Errorf("%s must be at least %s characters", field, e.Param())
				case "max":
					return fmt.Errorf("%s must be at most %s characters", field, e.Param())
				default:
					return fmt.Errorf("%s is invalid", field)
				}
			}
		}
		return err
	}
	return nil
}
