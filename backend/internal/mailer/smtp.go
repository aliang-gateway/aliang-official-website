package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSenderFromConfigFile(configPath string) (*SMTPSender, error) {
	return nil, fmt.Errorf("deprecated: use NewSMTPSender")
}

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port <= 0 || strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Password) == "" || strings.TrimSpace(cfg.From) == "" {
		return nil, fmt.Errorf("smtp config requires host, port, username, password, from")
	}

	return &SMTPSender{cfg: cfg}, nil
}

func (s *SMTPSender) Send(_ context.Context, toEmail, subject, body string) error {
	toEmail = strings.TrimSpace(toEmail)
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if toEmail == "" || subject == "" || body == "" {
		return fmt.Errorf("to_email, subject and body are required")
	}

	addr := s.cfg.Host + ":" + strconv.Itoa(s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	fromHeader := s.cfg.From
	if strings.TrimSpace(s.cfg.FromName) != "" {
		fromHeader = fmt.Sprintf("%s <%s>", strings.TrimSpace(s.cfg.FromName), s.cfg.From)
	}

	msg := "From: " + fromHeader + "\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body + "\r\n"

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{toEmail}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send mail: %w", err)
	}

	return nil
}
