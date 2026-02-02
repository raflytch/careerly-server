package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	authService domain.AuthService
}

func NewAuthHandler(authService domain.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error {
	state := generateState()
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	url := h.authService.GetGoogleLoginURL(state)
	return c.Redirect(url)
}

func (h *AuthHandler) GoogleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		return response.BadRequest(c, "missing authorization code")
	}

	authResponse, err := h.authService.HandleGoogleCallback(c.UserContext(), code)
	if err != nil {
		if errors.Is(err, domain.ErrUserDeleted) {
			return response.Error(c, fiber.StatusConflict, err.Error())
		}
		if errors.Is(err, service.ErrUserNotActive) {
			return response.Forbidden(c, err.Error())
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "login successful", authResponse)
}

func (h *AuthHandler) RequestRestoreOTP(c *fiber.Ctx) error {
	var req domain.OTPRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Email == "" {
		return response.BadRequest(c, "email is required")
	}

	otpResponse, err := h.authService.RequestRestoreOTP(c.UserContext(), req.Email)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNoDeletedUserFound):
			return response.NotFound(c, err.Error())
		case errors.Is(err, domain.ErrUserAlreadyActive):
			return response.BadRequest(c, err.Error())
		case errors.Is(err, domain.ErrOTPAlreadySent):
			return response.Error(c, fiber.StatusTooManyRequests, err.Error())
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusOK, "OTP sent successfully", otpResponse)
}

func (h *AuthHandler) VerifyRestoreOTP(c *fiber.Ctx) error {
	var req domain.OTPVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Email == "" {
		return response.BadRequest(c, "email is required")
	}

	if req.OTP == "" || len(req.OTP) != 6 {
		return response.BadRequest(c, "OTP must be 6 digits")
	}

	restoreResponse, err := h.authService.VerifyRestoreOTP(c.UserContext(), req.Email, req.OTP)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidOTP):
			return response.BadRequest(c, err.Error())
		case errors.Is(err, domain.ErrNoDeletedUserFound):
			return response.NotFound(c, err.Error())
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusOK, "account restored successfully", restoreResponse)
}

func (h *AuthHandler) ResendRestoreOTP(c *fiber.Ctx) error {
	var req domain.OTPRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.Email == "" {
		return response.BadRequest(c, "email is required")
	}

	otpResponse, err := h.authService.ResendRestoreOTP(c.UserContext(), req.Email)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNoDeletedUserFound):
			return response.NotFound(c, err.Error())
		case errors.Is(err, domain.ErrUserAlreadyActive):
			return response.BadRequest(c, err.Error())
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusOK, "OTP resent successfully", otpResponse)
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
