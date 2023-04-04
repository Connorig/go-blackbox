package email

import "testing"

func TestIndex(t *testing.T) {

	conf := MailConnConf{
		User:  "connor-g@qq.com",
		Pass:  "jkgolslkqlnsdiid",
		Host:  "smtp.qq.com",
		Alias: "connor",
	}

	err := GetClient(&conf).SendMail([]string{"connor@thingple.com"}, "测试", "hello", "", "")
	if err != nil {
		t.Error(err)
	}
}
