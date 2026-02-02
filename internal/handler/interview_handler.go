package handler

import (
	"errors"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/middleware"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type InterviewHandler struct {
	interviewService domain.InterviewService
	quotaService     domain.QuotaService
}

func NewInterviewHandler(interviewService domain.InterviewService, quotaService domain.QuotaService) *InterviewHandler {
	return &InterviewHandler{
		interviewService: interviewService,
		quotaService:     quotaService,
	}
}

func (h *InterviewHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	var req domain.CreateInterviewRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if err := validateInterviewRequest(&req); err != nil {
		return response.BadRequest(c, err.Error())
	}

	result, err := h.interviewService.Create(c.UserContext(), user.ID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveSubscription) {
			return response.Forbidden(c, "no active subscription found")
		}
		if errors.Is(err, service.ErrQuotaExceeded) {
			return response.Forbidden(c, "interview quota exceeded for this month")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusCreated, "interview created", result)
}

func (h *InterviewHandler) GetByID(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid interview id")
	}

	interview, err := h.interviewService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrInterviewNotFound) {
			return response.NotFound(c, "interview not found")
		}
		if errors.Is(err, service.ErrInterviewUnauthorized) {
			return response.Forbidden(c, "unauthorized access to interview")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "interview retrieved", interview)
}

func (h *InterviewHandler) GetMyInterviews(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	result, err := h.interviewService.GetByUserID(c.UserContext(), user.ID, page, limit)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "interviews retrieved", result)
}

func (h *InterviewHandler) SubmitAnswers(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid interview id")
	}

	var req domain.SubmitAnswerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if err := validateSubmitAnswerRequest(&req); err != nil {
		return response.BadRequest(c, err.Error())
	}

	result, err := h.interviewService.SubmitAnswers(c.UserContext(), user.ID, id, &req)
	if err != nil {
		if errors.Is(err, service.ErrInterviewNotFound) {
			return response.NotFound(c, "interview not found")
		}
		if errors.Is(err, service.ErrInterviewUnauthorized) {
			return response.Forbidden(c, "unauthorized access to interview")
		}
		if errors.Is(err, service.ErrInterviewCompleted) {
			return response.BadRequest(c, "interview already completed")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "answers submitted and evaluated", result)
}

func (h *InterviewHandler) Delete(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid interview id")
	}

	if err := h.interviewService.Delete(c.UserContext(), user.ID, id); err != nil {
		if errors.Is(err, service.ErrInterviewNotFound) {
			return response.NotFound(c, "interview not found")
		}
		if errors.Is(err, service.ErrInterviewUnauthorized) {
			return response.Forbidden(c, "unauthorized access to interview")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "interview deleted", nil)
}

func validateInterviewRequest(req *domain.CreateInterviewRequest) error {
	validate := validator.New()
	return validate.Struct(req)
}

func validateSubmitAnswerRequest(req *domain.SubmitAnswerRequest) error {
	validate := validator.New()
	return validate.Struct(req)
}
