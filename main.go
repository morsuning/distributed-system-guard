package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"system-usability-detection/internal/config"
	"system-usability-detection/internal/util"
	"system-usability-detection/internal/version"
	"system-usability-detection/pkg/metrics"
	"system-usability-detection/pkg/server"
	"system-usability-detection/pkg/status_check"
	"time"
)

func NewService() {
	brainServer, err := server.NewBrainServer()
	if err != nil {
		util.Logger.Error("init split brain brainServer failed:%v", err)
		return
	}
	si := config.GetCheckMode()
	si = append(si, status_check.NewKeepAlivedCheckImpl(""))
	brainServer.Start(context.Background(), si)

	router := mux.NewRouter()
	router.Methods(http.MethodGet).Path("/check").HandlerFunc(brainServer.BrainCheckHandler)
	// pprof
	router.Methods(http.MethodGet).Path("/debug/pprof/").HandlerFunc(pprof.Index)
	router.Methods(http.MethodGet).Path("/debug/pprof/cmdline").HandlerFunc(pprof.Cmdline)
	router.Methods(http.MethodGet).Path("/debug/pprof/profile").HandlerFunc(pprof.Profile)
	router.Methods(http.MethodGet).Path("/debug/pprof/symbol").HandlerFunc(pprof.Symbol)
	router.Methods(http.MethodGet).Path("/debug/pprof/trace").HandlerFunc(pprof.Trace)

	srv := &http.Server{
		Addr:         ":12345",
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
		IdleTimeout:  time.Second * 120,
		Handler:      router,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			util.Logger.Error("http brainServer listen failed:%v", err)
			fmt.Println(err)
		}
	}()
	util.Logger.Info("started service")
	//开启指标采集
	go metrics.StartMetricsServer(":12346")
	//开启push gateway推送
	go metrics.LoopPushingMetric("split_brain_check", "", 0)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	<-c
	err = srv.Shutdown(context.Background())
	if err != nil {
		return
	}
	util.Logger.Info("shutdown service")
}

func main() {
	configPath := flag.String("config", "./config.yml", "config file")
	versionInfo := flag.Bool("version", false, "print version")
	supportType := flag.Bool("support", false, "print support check types")

	if *versionInfo {
		log.Printf("version: %s", version.Version)
		return
	}

	if *supportType {
		log.Println("support check types: ")
		for _, v := range status_check.GetAllSupportType() {
			log.Printf("\t %s \n", v)
		}
		return
	}

	// 解析配置文件
	config.ParseConfig(*configPath)
	NewService()
}
