package version

import (
	"encoding/json"
	"fmt"
	"runtime"
)

/**
* @Author: Connor
* @Date:   24.6.7 11:44
* @Description:
 */

var (
	AppName  = ""
	Version  = ""
	Commit   = ""
	Build    = ""
	Compiler = ""
)

// Print 输出可读版本信息。
func Print() {
	fmt.Println("***********************************")
	fmt.Printf("Name     :%s\n", AppName)
	fmt.Printf("Version  :%s\n", Version)
	fmt.Printf("Commit   :%s\n", Commit)
	fmt.Printf("Build    :%s\n", Build)
	fmt.Printf("Compiler :%s\n", Compiler)
	fmt.Printf("Runtime  :%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("***********************************")
}

// info 是 JSON 输出使用的版本信息结构。
type info struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Build    string `json:"build"`
	Compiler string `json:"compiler"`
	Runtime  string `json:"runtime"`
}

// JSON 输出机器可读的版本信息，便于发布与诊断脚本解析。
func JSON() string {
	payload, err := json.Marshal(info{
		Name:     AppName,
		Version:  Version,
		Commit:   Commit,
		Build:    Build,
		Compiler: Compiler,
		Runtime:  runtime.GOOS + "/" + runtime.GOARCH,
	})
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(payload)
}
