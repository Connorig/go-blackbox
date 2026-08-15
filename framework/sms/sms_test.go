package sms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestNewClientInvalid 非法配置报错。
func TestNewClientInvalid(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("empty access key must fail")
	}
}

// TestSignRPCDeterministic 签名可复现 + 参数变化签名变化。
func TestSignRPCDeterministic(t *testing.T) {
	params := map[string]string{
		"Action":       "SendSms",
		"PhoneNumbers": "13800138000",
		"SignName":     "测试签名",
	}
	first := signRPC("secret", params)
	second := signRPC("secret", params)
	if first != second {
		t.Fatal("signature must be deterministic")
	}
	if first == "" {
		t.Fatal("signature must not be empty")
	}
	params["PhoneNumbers"] = "13900139000"
	if signRPC("secret", params) == first {
		t.Fatal("param change must change signature")
	}
}

// TestPercentEncode 特殊字符编码。
func TestPercentEncode(t *testing.T) {
	if got := percentEncode("a b+c*d~e"); got != "a%20b%2Bc%2Ad~e" {
		t.Fatalf("encode = %q", got)
	}
	if got := percentEncode("中文"); !strings.Contains(got, "%") {
		t.Fatalf("chinese must be encoded: %q", got)
	}
}

// TestSendAgainstMock 用 mock 服务验证请求参数与签名头。
func TestSendAgainstMock(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Code":"OK","Message":"OK","RequestId":"req-1","BizId":"biz-1"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		AccessKeyID:     "LTAI-test",
		AccessKeySecret: "secret-key",
		SignName:        "测试签名",
		TemplateCode:    "SMS_123456",
		Endpoint:        server.URL,
	})
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	response, err := client.Send(context.Background(), SendRequest{
		PhoneNumbers:  "13800138000",
		TemplateParam: map[string]string{"code": "123456"},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if !response.IsSuccess() {
		t.Fatalf("expected success, got %+v", response)
	}
	// 请求参数齐全
	for _, want := range []string{
		"Action=SendSms",
		"Version=2017-05-25",
		"PhoneNumbers=13800138000",
		"SignatureMethod=HMAC-SHA1",
		"TemplateParam=",
		"Signature=",
	} {
		if !strings.Contains(receivedQuery, want) {
			t.Errorf("query missing %q: %s", want, receivedQuery)
		}
	}
}

// TestSendMockError 阿里云错误码透传。
func TestSendMockError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Code":"isv.MOBILE_NUMBER_ILLEGAL","Message":"号码非法"}`))
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		AccessKeyID: "LTAI-test", AccessKeySecret: "secret",
		SignName: "测试", TemplateCode: "SMS_1",
		Endpoint: server.URL,
	})
	response, err := client.Send(context.Background(), SendRequest{PhoneNumbers: "bad"})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if response.IsSuccess() {
		t.Fatal("must not be success")
	}
	if response.Code != "isv.MOBILE_NUMBER_ILLEGAL" {
		t.Fatalf("code = %q", response.Code)
	}
}

// TestSendValidation 参数校验。
func TestSendValidation(t *testing.T) {
	client, _ := NewClient(Config{AccessKeyID: "k", AccessKeySecret: "s", SignName: "n", TemplateCode: "t"})
	if _, err := client.Send(context.Background(), SendRequest{}); err == nil {
		t.Fatal("empty phone must fail")
	}
	clientNoTemplate, _ := NewClient(Config{AccessKeyID: "k", AccessKeySecret: "s"})
	if _, err := clientNoTemplate.Send(context.Background(), SendRequest{PhoneNumbers: "13800138000"}); err == nil {
		t.Fatal("missing sign/template must fail")
	}
}

// TestSendReal 真实发送(环境变量启用)。
func TestSendReal(t *testing.T) {
	accessKeyID := os.Getenv("GO_BLACKBOX_SMS_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("GO_BLACKBOX_SMS_ACCESS_KEY_SECRET")
	phone := os.Getenv("GO_BLACKBOX_SMS_PHONE")
	signName := os.Getenv("GO_BLACKBOX_SMS_SIGN_NAME")
	templateCode := os.Getenv("GO_BLACKBOX_SMS_TEMPLATE_CODE")
	if accessKeyID == "" || phone == "" {
		t.Skip("sms real test not configured")
	}
	client, err := NewClient(Config{
		AccessKeyID: accessKeyID, AccessKeySecret: accessKeySecret,
		SignName: signName, TemplateCode: templateCode,
	})
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	response, err := client.Send(context.Background(), SendRequest{
		PhoneNumbers:  phone,
		TemplateParam: map[string]string{"code": "888888"},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	t.Logf("sms response: code=%s message=%s requestId=%s", response.Code, response.Message, response.RequestID)
	if response.Code != "OK" {
		t.Fatalf("send failed: %+v", response)
	}
}
