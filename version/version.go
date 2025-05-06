package version

import (
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
