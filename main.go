package main

import (
	"flag"
	"log"
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

}

func NewService() {

}