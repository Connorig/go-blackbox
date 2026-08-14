package email

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSendMailWithEnvironmentConfig 验证真实 SMTP 发送；需要环境变量提供凭据，默认跳过。
// 设置 GO_BLACKBOX_SMTP_USER / GO_BLACKBOX_SMTP_PASS / GO_BLACKBOX_SMTP_HOST /
// GO_BLACKBOX_SMTP_TO 后才会执行，禁止在仓库中硬编码任何邮箱凭据。
func TestSendMailWithEnvironmentConfig(t *testing.T) {
	user := os.Getenv("GO_BLACKBOX_SMTP_USER")
	pass := os.Getenv("GO_BLACKBOX_SMTP_PASS")
	host := os.Getenv("GO_BLACKBOX_SMTP_HOST")
	mailTo := os.Getenv("GO_BLACKBOX_SMTP_TO")
	if user == "" || pass == "" || host == "" || mailTo == "" {
		t.Skip("SMTP integration test requires GO_BLACKBOX_SMTP_USER/PASS/HOST/TO environment variables")
	}

	conf := MailConnConf{
		User:  user,
		Pass:  pass,
		Host:  host,
		Alias: "go-blackbox-test",
	}
	if err := GetClient(&conf).SendMail([]string{mailTo}, "go-blackbox test", "hello", "", ""); err != nil {
		t.Fatalf("send mail failed: %v", err)
	}
}

// TestGetClientDefaults 验证端口默认值与显式端口保留。
func TestGetClientDefaults(t *testing.T) {
	client := GetClient(&MailConnConf{User: "u", Pass: "p", Host: "smtp.example.com"})
	if client.port != DefaultSMTPPort {
		t.Fatalf("expected default port %d, got %d", DefaultSMTPPort, client.port)
	}
	client = GetClient(&MailConnConf{User: "u", Pass: "p", Host: "smtp.example.com", Port: 587})
	if client.port != 587 {
		t.Fatalf("expected explicit port 587, got %d", client.port)
	}
}

// TestSendMailRejectsUnconfiguredClient 验证未配置主机/用户时返回错误。
func TestSendMailRejectsUnconfiguredClient(t *testing.T) {
	client := GetClient(nil)
	if err := client.SendMail([]string{"a@b.com"}, "s", "b", "", ""); err == nil {
		t.Fatal("unconfigured client must return an error")
	}
}

// TestSendMailRejectsEmptyRecipients 验证空收件人返回错误。
func TestSendMailRejectsEmptyRecipients(t *testing.T) {
	client := GetClient(&MailConnConf{User: "u", Pass: "p", Host: "smtp.example.com"})
	if err := client.SendMail(nil, "s", "b", "", ""); err == nil {
		t.Fatal("empty recipients must return an error")
	}
}

// TestValidateAttachment 验证附件参数、存在性与目录校验。
func TestValidateAttachment(t *testing.T) {
	client := GetClient(&MailConnConf{User: "u", Pass: "p", Host: "smtp.example.com"})

	if err := client.validateAttachment("", ""); err != nil {
		t.Fatalf("empty attachment must be allowed: %v", err)
	}
	if err := client.validateAttachment("a.txt", ""); err == nil {
		t.Fatal("fileName without filePath must be rejected")
	}
	if err := client.validateAttachment("", "a.txt"); err == nil {
		t.Fatal("filePath without fileName must be rejected")
	}
	if err := client.validateAttachment("a.txt", filepath.Join(os.TempDir(), "not-exist-file-xyz")); err == nil {
		t.Fatal("missing attachment file must be rejected")
	}
	if err := client.validateAttachment("dir", t.TempDir()); err == nil {
		t.Fatal("directory attachment must be rejected")
	}
}
