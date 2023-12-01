package config

import (
	"fmt"
	"log"
	"strconv"

	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	keepAlivedPrefix = "/keepalived/"
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
	Instances          []*vrrpInstance
	etcdPoints, checks []string
	dial, ttl          int
}

type vrrpInstance struct {
	priority           int
	virtualIP, localIP string
	// etcd leaseID
	LeaseID     clientv3.LeaseID
	KeepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
	// 标记leaseId和KeepAliveCh是否残留，register失败时会残留
	HaveResidualInfo bool
}

func (v *vrrpInstance) GenerateKV() (string, string) {
	return fmt.Sprintf("%s%s/%s", keepAlivedPrefix, v.virtualIP, v.localIP), strconv.Itoa(v.priority)
}

type GlobalConfig struct {
	VrrpInstances    *vrrpInstances
	InstancesCount   int
	VrrpNetInterface string
}

var GlobalConfigInstance *GlobalConfig

func ParseConfig(path string) {
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
