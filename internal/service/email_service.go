package service

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/raflytch/careerly-server/internal/config"
	"github.com/raflytch/careerly-server/internal/domain"
)

type emailService struct {
	cfg config.SMTPConfig
}

func NewEmailService(cfg config.SMTPConfig) domain.EmailService {
	return &emailService{cfg: cfg}
}

func (s *emailService) sendEmail(to, subject, body string) error {
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.cfg.From, to, subject, body)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}

func (s *emailService) SendOTP(_ context.Context, email, otp string) error {
	subject := "Your Account Restoration OTP - Careerly"
	body := fmt.Sprintf(
		"Careerly - Account Restoration\n\n"+
			"We received a request to restore your deleted Careerly account.\n\n"+
			"Your OTP Code: %s\n\n"+
			"This OTP will expire in 15 minutes. Do not share this code with anyone.\n\n"+
			"If you did not request this restoration, please ignore this email.\n\n"+
			"Careerly Team", otp)

	return s.sendEmail(email, subject, body)
}

func (s *emailService) SendDeleteOTP(_ context.Context, email, otp string) error {
	subject := "Account Deletion Confirmation OTP - Careerly"
	body := fmt.Sprintf(
		"Careerly - Account Deletion\n\n"+
			"We received a request to delete your Careerly account.\n\n"+
			"Your OTP Code: %s\n\n"+
			"WARNING: This action is irreversible. Your account and all associated data will be permanently deleted.\n"+
			"This OTP will expire in 15 minutes.\n\n"+
			"If you did not request this deletion, please ignore this email and secure your account immediately.\n\n"+
			"Careerly Team", otp)

	return s.sendEmail(email, subject, body)
}
