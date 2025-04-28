package company

import (
	"fmt"
	"log"
	"net/smtp"
)

type MailService interface {
	SendTempPassword(email, password string) error
}

type SMTPMailer struct {
	host     string
	port     string
	username string
	password string
}

func NewSMTPMailer(host, port, username, password string) *SMTPMailer {
	return &SMTPMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
	}
}

func (m *SMTPMailer) SendTempPassword(email, password string) error {
	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Ваш временный пароль\r\nMIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\nПароль: %s",
		m.username,
		email,
		password,
	)
	err := smtp.SendMail(
		fmt.Sprintf("%s:%s", m.host, m.port),
		auth,
		m.username,
		[]string{email},
		[]byte(msg),
	)

	if err != nil {
		log.Printf("SMTP error: %v", err)
		return err
	}
	return nil
}
