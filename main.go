package main

import (
	"flag"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	err := InitDB()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	s := flag.String("f", "", "批量导入翻译的 JSON 文件")
	flag.Parse()

	if *s != "" {
		LoadFromJson(*s)
	} else {
		StartServer()
	}
}
