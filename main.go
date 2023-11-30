package main

import (
	"flag"
	"log"

	"split_brain_check/internal/util"
)

func main() {
	configPath := flag.String("config", "./config.yml", "config file")
	versionInfo := flag.Bool("version", false, "print version")
	supportType := flag.Bool("support", false, "print support check types")

	if *versionInfo {
		log.Printf("version: %d", versionInfo)
		return
	}

	if *supportType {

	}

	// 解析配置文件
	util.ParseConfig(*configPath)
}

func NewService() {

}
