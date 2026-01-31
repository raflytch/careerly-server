package handler

import (
	"errors"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type PlanHandler struct {
	planService domain.PlanService
}

func NewPlanHandler(planService domain.PlanService) *PlanHandler {
	return &PlanHandler{
		planService: planService,
	}
}

func (h *PlanHandler) Create(c *fiber.Ctx) error {
	var req domain.CreatePlanRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	plan, err := h.planService.Create(c.UserContext(), &req)
	if err != nil {
		if errors.Is(err, service.ErrPlanNameExists) {
			return response.BadRequest(c, "plan name already exists")
		}
		if errors.Is(err, service.ErrInvalidPlanData) {
			return response.BadRequest(c, "name and display_name are required")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusCreated, "plan created", plan)
}

func (h *PlanHandler) GetByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid plan id")
	}

	plan, err := h.planService.GetByID(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, service.ErrPlanNotFound) {
			return response.NotFound(c, "plan not found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "plan retrieved", plan)
}

func (h *PlanHandler) GetAll(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	includeInactive := c.QueryBool("include_inactive", false)

	result, err := h.planService.GetAll(c.UserContext(), page, limit, includeInactive)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "plans retrieved", result)
}

func (h *PlanHandler) Update(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid plan id")
	}

	var req domain.UpdatePlanRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	plan, err := h.planService.Update(c.UserContext(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrPlanNotFound) {
			return response.NotFound(c, "plan not found")
		}
		if errors.Is(err, service.ErrPlanNameExists) {
			return response.BadRequest(c, "plan name already exists")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "plan updated", plan)
}

func (h *PlanHandler) Delete(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid plan id")
	}

	if err := h.planService.Delete(c.UserContext(), id); err != nil {
		if errors.Is(err, service.ErrPlanNotFound) {
			return response.NotFound(c, "plan not found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "plan deleted", nil)
}
