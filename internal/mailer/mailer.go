package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

//go:embed assets/logo.png
var logoPNG []byte

// logoContentID is the Content-ID used to reference the inline logo image
// from the HTML templates via a cid: URI.
const logoContentID = "raddigo-logo"

// Mailer sends transactional emails.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, otp string) error
	SendPasswordResetEmail(ctx context.Context, to, otp string) error
	SendBookingOTP(ctx context.Context, to, otp string) error
}

// LogMailer is a development Mailer that logs emails instead of sending them.
type LogMailer struct {
	logger *slog.Logger
}

// NewLogMailer creates a LogMailer.
func NewLogMailer(logger *slog.Logger) *LogMailer {
	return &LogMailer{logger: logger}
}

// SendVerificationEmail logs the verification OTP for the recipient.
func (m *LogMailer) SendVerificationEmail(_ context.Context, to, otp string) error {
	m.logger.Info("verification email",
		"to", to,
		"otp", otp,
	)
	return nil
}

// SendPasswordResetEmail logs the password reset OTP for the recipient.
func (m *LogMailer) SendPasswordResetEmail(_ context.Context, to, otp string) error {
	m.logger.Info("password reset email",
		"to", to,
		"otp", otp,
	)
	return nil
}

// SendBookingOTP logs the booking completion OTP for the recipient.
func (m *LogMailer) SendBookingOTP(_ context.Context, to, otp string) error {
	m.logger.Info("booking completion otp email",
		"to", to,
		"otp", otp,
	)
	return nil
}

// SMTPConfig holds the connection and sender details for SMTPMailer.
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

// SMTPMailer is a Mailer that sends real emails over SMTP.
type SMTPMailer struct {
	cfg    SMTPConfig
	logger *slog.Logger
}

// NewSMTPMailer creates an SMTPMailer.
func NewSMTPMailer(cfg SMTPConfig, logger *slog.Logger) *SMTPMailer {
	return &SMTPMailer{cfg: cfg, logger: logger}
}

// SendVerificationEmail sends the account verification OTP.
func (m *SMTPMailer) SendVerificationEmail(ctx context.Context, to, otp string) error {
	body, err := renderOTPEmail(otpEmailData{
		Title:  "Verify your email",
		Intro:  "Use the code below to verify your Raddigo account.",
		OTP:    otp,
		Footer: "If you didn't request this, you can safely ignore this email.",
	})
	if err != nil {
		return fmt.Errorf("render verification email: %w", err)
	}
	return m.send(ctx, to, "Verify your Raddigo account", body)
}

// SendPasswordResetEmail sends the password reset OTP.
func (m *SMTPMailer) SendPasswordResetEmail(ctx context.Context, to, otp string) error {
	body, err := renderOTPEmail(otpEmailData{
		Title:  "Reset your password",
		Intro:  "Use the code below to reset your Raddigo account password.",
		OTP:    otp,
		Footer: "If you didn't request this, you can safely ignore this email.",
	})
	if err != nil {
		return fmt.Errorf("render password reset email: %w", err)
	}
	return m.send(ctx, to, "Reset your Raddigo password", body)
}

// SendBookingOTP sends the booking completion OTP.
func (m *SMTPMailer) SendBookingOTP(ctx context.Context, to, otp string) error {
	body, err := renderOTPEmail(otpEmailData{
		Title:  "Booking completion code",
		Intro:  "Share the code below with your service partner to confirm booking completion.",
		OTP:    otp,
		Footer: "If you didn't request this, please contact Raddigo support.",
	})
	if err != nil {
		return fmt.Errorf("render booking otp email: %w", err)
	}
	return m.send(ctx, to, "Your Raddigo booking completion code", body)
}

// send connects to the configured SMTP server and delivers an HTML email
// with the app logo attached inline (referenced from the HTML via cid:).
func (m *SMTPMailer) send(ctx context.Context, to, subject, htmlBody string) error {
	from := m.cfg.FromAddress
	fromHeader := from
	if m.cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", m.cfg.FromName, from)
	}

	var msg bytes.Buffer
	mw := multipart.NewWriter(&msg)

	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/related; boundary=%s\r\n", mw.Boundary()))
	msg.WriteString("\r\n")

	if err := writeHTMLPart(mw, htmlBody); err != nil {
		return fmt.Errorf("write html part: %w", err)
	}
	if err := writeLogoPart(mw); err != nil {
		return fmt.Errorf("write logo part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprintf("%d", m.cfg.Port))
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)

	var sendErr error
	if m.cfg.Port == 465 {
		sendErr = sendImplicitTLS(addr, m.cfg.Host, auth, from, to, msg.Bytes())
	} else {
		sendErr = smtp.SendMail(addr, auth, from, []string{to}, msg.Bytes())
	}
	if sendErr != nil {
		return fmt.Errorf("smtp send: %w", sendErr)
	}

	m.logger.Info("email sent", "to", to, "subject", subject)
	return nil
}

func writeHTMLPart(mw *multipart.Writer, htmlBody string) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", `text/html; charset="UTF-8"`)
	part, err := mw.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(htmlBody))
	return err
}

// writeLogoPart attaches the embedded logo as an inline image, base64-encoded
// and wrapped at the standard 76-character line length.
func writeLogoPart(mw *multipart.Writer) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", "image/png")
	header.Set("Content-Transfer-Encoding", "base64")
	header.Set("Content-ID", fmt.Sprintf("<%s>", logoContentID))
	header.Set("Content-Disposition", `inline; filename="logo.png"`)
	part, err := mw.CreatePart(header)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(logoPNG)
	const lineLen = 76
	for i := 0; i < len(encoded); i += lineLen {
		end := i + lineLen
		if end > len(encoded) {
			end = len(encoded)
		}
		if _, err := part.Write([]byte(encoded[i:end] + "\r\n")); err != nil {
			return err
		}
	}
	return nil
}

// sendImplicitTLS delivers a message over an SMTP connection that is
// TLS-encrypted from the start (used for port 465), since smtp.SendMail only
// supports STARTTLS.
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

type otpEmailData struct {
	Title   string
	Intro   string
	OTP     string
	Footer  string
	LogoCID string
	Year    int
}

var otpEmailTemplate = template.Must(template.New("otpEmail").Parse(strings.TrimSpace(`
<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background-color:#f0fdf4;font-family:Arial,Helvetica,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f0fdf4;padding:24px 0;">
    <tr>
      <td align="center">
        <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;border:1px solid #dcfce7;">
          <tr>
            <td align="center" style="background-color:#ffffff;padding:24px 32px;border-bottom:4px solid #16a34a;">
              <img src="cid:{{.LogoCID}}" alt="Raddigo" width="56" height="56" style="display:block;" />
            </td>
          </tr>
          <tr>
            <td style="padding:32px;">
              <h1 style="margin:0 0 12px;font-size:20px;color:#14532d;">{{.Title}}</h1>
              <p style="margin:0 0 24px;font-size:14px;color:#374151;line-height:1.5;">{{.Intro}}</p>
              <div style="text-align:center;margin:0 0 24px;">
                <span style="display:inline-block;padding:12px 24px;background-color:#ecfdf5;border:1px solid #16a34a;border-radius:6px;font-size:28px;letter-spacing:6px;font-weight:bold;color:#065f46;">{{.OTP}}</span>
              </div>
              <p style="margin:0;font-size:12px;color:#6b7280;line-height:1.5;">{{.Footer}}</p>
            </td>
          </tr>
          <tr>
            <td align="center" style="background-color:#f0fdf4;padding:16px 32px;">
              <p style="margin:0;font-size:11px;color:#6b7280;">&copy; {{.Year}} Raddigo. All rights reserved.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`)))

func renderOTPEmail(data otpEmailData) (string, error) {
	data.LogoCID = logoContentID
	data.Year = time.Now().Year()
	var buf bytes.Buffer
	if err := otpEmailTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
