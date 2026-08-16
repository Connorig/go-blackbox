package email

import (
	"errors"
	"fmt"
	"mime"
	"os"

	"gopkg.in/gomail.v2"
)

// 默认 SMTP 端口与附件大小上限。
const (
	DefaultSMTPPort = 465 // 465 使用 SSL；587 使用 STARTTLS
	// MaxAttachmentSize 是附件大小上限（25MB）。
	MaxAttachmentSize = 25 * 1024 * 1024
)

// Client 封装 SMTP 发送配置。
type Client struct {
	user  string
	pass  string
	host  string
	alias string
	port  int
}

// GetClient 根据配置创建邮件客户端；nil 配置返回空客户端（SendMail 会报错）。
func GetClient(emailCong *MailConnConf) *Client {
	if emailCong == nil {
		return &Client{}
	}
	port := emailCong.Port
	if port <= 0 {
		port = DefaultSMTPPort
	}
	client := &Client{
		user:  emailCong.User,
		pass:  emailCong.Pass,
		host:  emailCong.Host,
		alias: emailCong.Alias,
		port:  port,
	}
	SetGlobal(client)
	return client
}

// SendMail 发送邮件。
// mailTo 支持多人发送；subject 为标题；body 为 HTML 正文。
// fileName 与 filePath 同时非空时附带附件（存在性、大小与目录校验）。
func (emailC *Client) SendMail(mailTo []string, subject, body, fileName, filePath string) error {
	if emailC == nil || emailC.host == "" || emailC.user == "" {
		return errors.New("mail client is not configured (host and user are required)")
	}
	if len(mailTo) == 0 {
		return errors.New("mailTo length must not be zero")
	}
	if err := emailC.validateAttachment(fileName, filePath); err != nil {
		return err
	}

	m := gomail.NewMessage(gomail.SetEncoding(gomail.Base64))
	m.SetHeader("From", m.FormatAddress(emailC.user, emailC.alias))
	m.SetHeader("To", mailTo...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	if len(fileName) > 0 && len(filePath) > 0 {
		m.Attach(filePath,
			gomail.Rename(fileName),
			gomail.SetHeader(map[string][]string{
				"Content-Disposition": {
					fmt.Sprintf(`attachment; filename="%s"`, mime.QEncoding.Encode("UTF-8", fileName)),
				},
			}),
		)
	}

	// 端口 465 自动启用 SSL；其他端口（如 587）使用 STARTTLS。
	// gomail 内置 10 秒连接超时。
	d := gomail.NewDialer(emailC.host, emailC.port, emailC.user, emailC.pass)
	d.SSL = emailC.port == 465
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("send mail via %s:%d: %w", emailC.host, emailC.port, err)
	}
	return nil
}

// validateAttachment 校验附件参数、存在性与大小。
func (emailC *Client) validateAttachment(fileName, filePath string) error {
	if fileName == "" && filePath == "" {
		return nil
	}
	if fileName == "" || filePath == "" {
		return errors.New("attachment requires both fileName and filePath")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("attachment %q: %w", filePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("attachment %q is a directory", filePath)
	}
	if info.Size() > MaxAttachmentSize {
		return fmt.Errorf("attachment %q exceeds max size %d bytes", filePath, MaxAttachmentSize)
	}
	return nil
}
