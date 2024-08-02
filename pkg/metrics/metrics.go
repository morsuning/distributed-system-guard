package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"log"
	"net/http"
	"os"
	"strings"
	"system-usability-detection/internal/util"
	"time"
)

const nameSpace = "system-usability-detection"

var (
	Gather = prometheus.NewRegistry()

	CacheCheckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nameSpace,
			Subsystem: "power_cache",
			Name:      "cache_check_updates",
			Help:      "Counter of power_cache updates.",
		}, []string{"type"}) // power_cache检测计数counter

	FrontInterfaceCheckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nameSpace,
			Subsystem: "front_interface",
			Name:      "front_interface_check_updates",
			Help:      "Counter of front_interface updates.",
		}, []string{"type"}) // 业务网卡检测计数counter

	NasCheckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nameSpace,
			Subsystem: "nas",
			Name:      "nas_check_updates",
			Help:      "Counter of nas updates.",
		}, []string{"type"}) // nas检测计数counter

	ServiceCheckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nameSpace,
			Subsystem: "Service",
			Name:      "Service_check_updates",
			Help:      "Counter of Service updates.",
		}, []string{"type"}) // Service检测计数counter

	NfsCheckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nameSpace,
			Subsystem: "nfs",
			Name:      "nfs_check_updates",
			Help:      "Counter of nfs updates.",
		}, []string{"type"}) // nfs检测计数counter

	SambaCheckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nameSpace,
			Subsystem: "samba",
			Name:      "samba_check_updates",
			Help:      "Counter of samba updates.",
		}, []string{"type"}) // samba检测计数counter

	ExecuteTimeOutGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: nameSpace,
			Subsystem: "server",
			Name:      "is_timeout",
			Help:      "execute allcheck is timeout",
		}) // 检查模块超时gauge
	RequestHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nameSpace,
			Subsystem: "request",
			Name:      "request_seconds",
			Help:      "Bucketed histogram of client request processing time.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 24),
		}, []string{"type", "bucket"}) // 客户端请求时间分布histogram
)

func init() {
	Gather.MustRegister(CacheCheckCounter)
	Gather.MustRegister(FrontInterfaceCheckCounter)
	Gather.MustRegister(NasCheckCounter)
	Gather.MustRegister(ServiceCheckCounter)
	Gather.MustRegister(NfsCheckCounter)
	Gather.MustRegister(ExecuteTimeOutGauge)
	Gather.MustRegister(RequestHistogram)

	Gather.MustRegister(collectors.NewGoCollector())
	Gather.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

}

// LoopPushingMetric 循环推送push gateway
func LoopPushingMetric(name, addr string, intervalSeconds int) {
	if addr == "" || intervalSeconds == 0 {
		return
	}
	var instance string
	hostname, err := os.Hostname()
	if err != nil {
		instance = "unknown"
	} else {
		instance = hostname
	}
	util.Logger.Info("%s server sends metrics to %s every %d seconds", name, addr, intervalSeconds)
	pusher := push.New(addr, name).Gatherer(Gather).Grouping("instance", instance)
	for {
		err := pusher.Push()
		if err != nil && !strings.HasPrefix(err.Error(), "unexpected status code 200") {
			util.Logger.Info("could not push metrics to prometheus push gateway %s: %v", addr, err)
		}
		if intervalSeconds <= 0 {
			intervalSeconds = 15
		}
		time.Sleep(time.Duration(intervalSeconds) * time.Second)
	}
}

// StartMetricsServer 开启指标服务
func StartMetricsServer(address string) {
	if address == "" {
		return
	}
	http.Handle("/metrics", promhttp.HandlerFor(Gather, promhttp.HandlerOpts{}))
	log.Fatal(http.ListenAndServe(address, nil))
}
