package handler

import (
	"errors"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/middleware"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/imagekit"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService    domain.UserService
	imagekitClient *imagekit.Client
}

func NewUserHandler(userService domain.UserService, imagekitClient *imagekit.Client) *UserHandler {
	return &UserHandler{
		userService:    userService,
		imagekitClient: imagekitClient,
	}
}

type UpdateUserRequest struct {
	Name string `json:"name"`
}

func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	profile, err := h.userService.GetProfile(c.UserContext(), user.ID)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "profile retrieved", profile)
}

func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid user id")
	}

	user, err := h.userService.GetByID(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return response.NotFound(c, "user not found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "user retrieved", user)
}

func (h *UserHandler) GetAll(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	result, err := h.userService.GetAll(c.UserContext(), page, limit)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "users retrieved", result)
}

func (h *UserHandler) Update(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	contentType := c.Get("Content-Type")
	if contentType != "" && len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {

		file, err := c.FormFile("avatar")
		if err != nil {
			return response.BadRequest(c, "avatar file is required")
		}

		if err := h.imagekitClient.ValidateImage(file); err != nil {
			return response.BadRequest(c, err.Error())
		}

		uploadResult, err := h.imagekitClient.UploadFile(c.UserContext(), file, "avatars")
		if err != nil {
			return response.InternalError(c, "failed to upload avatar: "+err.Error())
		}

		updatedUser, err := h.userService.UpdateAvatar(c.UserContext(), user.ID, uploadResult.URL)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				return response.NotFound(c, "user not found")
			}
			return response.InternalError(c, err.Error())
		}

		return response.Success(c, fiber.StatusOK, "avatar updated", updatedUser)
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Name == "" {
		return response.BadRequest(c, "name is required")
	}

	updatedUser, err := h.userService.Update(c.UserContext(), user.ID, req.Name)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return response.NotFound(c, "user not found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "user updated", updatedUser)
}

func (h *UserHandler) Delete(c *fiber.Ctx) error {
	currentUser := middleware.GetUserFromContext(c)
	if currentUser == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid user id")
	}

	if currentUser.ID == id {
		return response.BadRequest(c, "cannot delete your own account")
	}

	err = h.userService.Delete(c.UserContext(), id, currentUser.Role)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return response.NotFound(c, "user not found")
		}
		if errors.Is(err, service.ErrForbiddenAction) {
			return response.Forbidden(c, "only admin can delete users")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "user deleted", nil)
}

func (h *UserHandler) RequestDeleteOTP(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	otpResponse, err := h.userService.RequestDeleteOTP(c.UserContext(), user)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrCannotDeleteAdmin):
			return response.Forbidden(c, err.Error())
		case errors.Is(err, domain.ErrOTPAlreadySent):
			return response.Error(c, fiber.StatusTooManyRequests, err.Error())
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusOK, "OTP sent successfully", otpResponse)
}

func (h *UserHandler) VerifyDeleteOTP(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	var req domain.OTPVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.OTP == "" || len(req.OTP) != 6 {
		return response.BadRequest(c, "OTP must be 6 digits")
	}

	deleteResponse, err := h.userService.VerifyDeleteOTP(c.UserContext(), user, req.OTP)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidOTP):
			return response.BadRequest(c, err.Error())
		case errors.Is(err, domain.ErrCannotDeleteAdmin):
			return response.Forbidden(c, err.Error())
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusOK, "account deleted successfully", deleteResponse)
}

func (h *UserHandler) ResendDeleteOTP(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "user not authenticated")
	}

	otpResponse, err := h.userService.ResendDeleteOTP(c.UserContext(), user)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrCannotDeleteAdmin):
			return response.Forbidden(c, err.Error())
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusOK, "OTP resent successfully", otpResponse)
}
