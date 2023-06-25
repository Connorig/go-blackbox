package zaplog

import (
	zaprotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"io"
	"strings"
	"time"
)

//func GetWriteSyncer() (zapcore.WriteSyncer, error) {
//	fileWriter, err := zaprotatelogs.New(
//		path.Join(CONFIG.Director, "---%Y-%m-%d.log"),
//		zaprotatelogs.WithLinkName(CONFIG.LinkName),
//		zaprotatelogs.WithMaxAge(7*24*time.Hour),
//		zaprotatelogs.WithRotationTime(24*time.Hour),
//	)
//	if CONFIG.LogInConsole {
//		return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(fileWriter)), err
//	}
//	return zapcore.AddSync(fileWriter), err
//}

// GetWriteSyncer2 创建写入文件流，根据日志级别写入不同文件中
// debug\info\warn\error 每个级别等级限制显示所属的内容
// info\warn\error
// warn\error
// error
func GetWriteSyncer2(filename string) io.Writer {
	hook, err := zaprotatelogs.New(
		strings.Replace(CONFIG.Director+filename, ".log", "", -1)+"-%Y%m%d%H.log",
		zaprotatelogs.WithLinkName(CONFIG.LinkName),  // 生成软链，指向最新日志文件
		zaprotatelogs.WithMaxAge(7*24*time.Hour),     // clear 最小分钟为小时
		zaprotatelogs.WithRotationTime(24*time.Hour), // rotate 最小为24小时轮询
	)
	if err != nil {
		panic(err)
	}
	return hook
}
