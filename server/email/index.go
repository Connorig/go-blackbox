package email

import (
	"errors"
	"fmt"
	"gopkg.in/gomail.v2"
	"mime"
)

/**
* @Author: Connor
* @Date:   23.3.31 16:25
* @Description: 发送邮件工具类
 */

type Client struct {
	user  string //发送人邮箱（邮箱以自己的为准）
	pass  string //发送人邮箱的密码，现在可能会需要邮箱 开启授权密码后在pass填写授权码 jkgolslkqlnsdiid
	host  string //邮箱服务器（此时用的是qq邮箱）
	alias string // 邮箱发送别名
}

// GetClient 获取邮件客户端
func GetClient(emailCong *MailConnConf) *Client {
	c := &Client{
		user:  emailCong.User,
		pass:  emailCong.Pass,
		host:  emailCong.Host,
		alias: emailCong.Alias,
	}
	return c
}

// SendMail 发送邮件
// mailTo 支持多人发送
// subject 信息主体
// fileName 附件名称
// filePath 文件路径
func (emailC *Client) SendMail(mailTo []string, subject, body, fileName, filePath string) error {
	if len(mailTo) == 0 {
		return errors.New("mailTo length must not be zero")
	}
	// 设置邮箱主体
	mailConn := map[string]string{
		"user": emailC.user, // 发送人邮箱（邮箱以自己的为准）
		"pass": emailC.pass, // 发送人邮箱的密码，现在可能会需要邮箱 开启授权密码后在pass填写授权码
		"host": emailC.host, // 邮箱服务器（此时用的是qq邮箱） "smtp.qq.com"
	}

	m := gomail.NewMessage(
		//发送文本时设置编码，防止乱码。 如果txt文本设置了之后还是乱码，那可以将原txt文本在保存时
		//就选择utf-8格式保存
		gomail.SetEncoding(gomail.Base64),
	)
	m.SetHeader("From", m.FormatAddress(mailConn["user"], emailC.alias)) // 添加别名
	m.SetHeader("To", mailTo...)                                         // 发送给用户(可以多个)
	m.SetHeader("Subject", subject)                                      // 设置邮件主题
	m.SetBody("text/html", body)                                         // 设置邮件正文

	//一个文件（加入发送一个 txt 文件）：/tmp/foo.txt，需要将这个文件以邮件附件的方式进行发送，同时指定附件名为：附件.txt
	//同时解决了文件名乱码问题
	if len(fileName) > 0 && len(filePath) > 0 {
		m.Attach(filePath,
			gomail.Rename(fileName), //重命名
			gomail.SetHeader(map[string][]string{
				"Content-Disposition": {
					fmt.Sprintf(`attachment; filename="%s"`, mime.QEncoding.Encode("UTF-8", fileName)),
				},
			}),
		)
	}
	/*
	   创建SMTP客户端，连接到远程的邮件服务器，需要指定服务器地址、端口号、用户名、密码，如果端口号为465的话，
	   自动开启SSL，这个时候需要指定TLSConfig
	*/

	d := gomail.NewDialer(mailConn["host"], 465, mailConn["user"], mailConn["pass"]) // 设置邮件正文
	//d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	err := d.DialAndSend(m)
	return err
}
