package util

import (
	"log"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Interface     string   `mapstructure:"interface"`
	EtcdEndpoints []string `mapstructure:"etcd"`
	Dial          int      `mapstructure:"dial"`
	TTL           int      `mapstructure:"ttl"`
	Instances     []struct {
		Name string `mapstructure:"name"`
		Vips []struct {
			Priority int    `mapstructure:"priority"`
			Vip      string `mapstructure:"vip"`
		} `mapstructure:"vips"`
		Check []string `mapstructure:"check"`
	} `mapstructure:"instances"`
}

type vrrpInstances struct {
	instances          []*vrrpInstance
	etcdPoints, checks []string
	dial, ttl          int
}

type vrrpInstance struct {
	priority           int
	virtualIP, localIP string
	// etcd leaseID
	leaseID     int64
	keepAliveCh <-chan *struct {
		// warps the protobuf message leaseKeep AliveResponse
	}
	// 标记leaseId和KeepAliveCh是否残留，register失败时会残留
	haveResidualInfo bool
}

type GlobalConfig struct {
	VrrpInstances    *vrrpInstances
	InstancesCount   int
	VrrpNetInterface string
}

var GlobalConfigInstance *GlobalConfig
var once sync.Once

func ParseConfig(path string) error {
	config := &Config{}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		log.Panicf("fatal error config file: %w", err)
	}
	if err := v.Unmarshal(&config); err != nil {
		log.Panicf("fatal error unmarshal config file: %w", err)
	}

}
