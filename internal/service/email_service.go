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

func (s *emailService) SendOTP(ctx context.Context, email, otp string) error {
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	subject := "Your Account Restoration OTP - Careerly"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f4f4; margin: 0; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 10px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);">
        <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; text-align: center;">
            <h1 style="color: #ffffff; margin: 0; font-size: 28px;">Careerly</h1>
            <p style="color: #e8e8e8; margin: 10px 0 0 0; font-size: 14px;">Account Restoration</p>
        </div>
        <div style="padding: 40px 30px;">
            <h2 style="color: #333333; margin: 0 0 20px 0; font-size: 22px;">Restore Your Account</h2>
            <p style="color: #666666; font-size: 16px; line-height: 1.6; margin: 0 0 25px 0;">
                We received a request to restore your deleted Careerly account. Use the OTP code below to complete the restoration process.
            </p>
            <div style="background-color: #f8f9fa; border: 2px dashed #667eea; border-radius: 8px; padding: 25px; text-align: center; margin: 30px 0;">
                <p style="color: #666666; font-size: 14px; margin: 0 0 10px 0;">Your OTP Code</p>
                <h1 style="color: #667eea; font-size: 42px; letter-spacing: 8px; margin: 0; font-weight: bold;">%s</h1>
            </div>
            <div style="background-color: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 25px 0; border-radius: 4px;">
                <p style="color: #856404; font-size: 14px; margin: 0;">
                    <strong>Important:</strong> This OTP will expire in <strong>15 minutes</strong>. Do not share this code with anyone.
                </p>
            </div>
            <p style="color: #666666; font-size: 14px; line-height: 1.6; margin: 25px 0 0 0;">
                If you did not request this restoration, please ignore this email. Your account will remain deleted.
            </p>
        </div>
        <div style="background-color: #f8f9fa; padding: 20px 30px; text-align: center; border-top: 1px solid #e9ecef;">
            <p style="color: #999999; font-size: 12px; margin: 0;">
                &copy; 2026 Careerly. All rights reserved.<br>
                This is an automated message, please do not reply.
            </p>
        </div>
    </div>
</body>
</html>
`, otp)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n%s", s.cfg.From, email, subject, body)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{email}, []byte(msg))
}
