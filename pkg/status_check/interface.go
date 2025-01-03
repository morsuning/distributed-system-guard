package status_check

import (
	"os"
	"strconv"
	"strings"
	"system-usability-detection/internal/config"
	"system-usability-detection/internal/util"
	"system-usability-detection/pkg/metrics"
	"time"
)

type StatusAction struct {
	Time   time.Time
	Name   string // 模块名称
	Status bool
	Extra  interface{} // 预留字段
}

// StatusInterface 状态检查接口
type StatusInterface interface {
	CheckStatus() StatusAction
	Name() string
}

// DefaultCheckModule 默认检测模块
var DefaultCheckModule = []StatusInterface{
	&FrontInterface{},
}

func GetAllSupportType() []string {
	var support []string
	for k := range GlobalMapping {
		support = append(support, k)
	}
	return support
}

// GlobalMapping 每新增一个检测模块,需要在这里添加映射
var GlobalMapping = map[string]StatusInterface{
	"nas":         &NasImpl{Address: "http://localhost:9999/api/status"}, // nas服务健康状态检测
	"nfs":         &NFSImpl{},                                            // nfsd服务健康状态检测
	"power_cache": &PowerCacheImpl{MountPoint: "/var/powercache"},        // powercache服务健康状态检测
	"service":     &OSSImpl{},                                            // service服务健康状态检测
	"samba":       &SambaImpl{},                                          // smbd服务健康状态检测
}

// 取globalMapping交集
func getValidCheck(a []string) []string {
	var ret []string
	for _, s := range a {
		if _, ok := GlobalMapping[s]; ok {
			ret = append(ret, s)
		}
	}
	return ret
}

// 编译检查
var _ StatusInterface = (*KeepAlivedCheckImpl)(nil)

// KeepAlivedCheckImpl 检查keepalived服务状态的实现
type KeepAlivedCheckImpl struct {
	PidFile string
}

// NewKeepAlivedCheckImpl 添加聚合方式
func NewKeepAlivedCheckImpl(pidFile string) StatusInterface {
	if pidFile == "" {
		pidFile = "/var/run/keepalived.pid"
	}
	return &KeepAlivedCheckImpl{
		PidFile: pidFile,
	}
}

// Name 那个模块的检测机制,这里对应模块名称
func (k *KeepAlivedCheckImpl) Name() string {
	return "keepalived"
}

func (k *KeepAlivedCheckImpl) CheckStatus() StatusAction {
	sa := StatusAction{
		Time:   time.Now(),
		Name:   k.Name(),
		Status: false,
	}
	data, err := os.ReadFile(k.PidFile)
	if err != nil {
		sa.Extra = err
		return sa
	}
	pid, err := strconv.Atoi(strings.TrimSuffix(string(data), "\n"))
	if err != nil {
		sa.Extra = err
		return sa
	}
	_, err = os.FindProcess(pid)
	if err != nil {
		sa.Extra = err
		return sa
	}
	sa.Status = true
	return sa
}

type FrontInterface struct{}

func (f *FrontInterface) Name() string {
	return "front_interface"
}

func (f *FrontInterface) CheckStatus() StatusAction {
	now := time.Now()
	defer func() {
		used := time.Since(now)
		util.Logger.Info("check %s used:%v", f.Name(), used)
	}()
	metrics.FrontInterfaceCheckCounter.WithLabelValues("total").Inc()
	sa := StatusAction{
		Time:   time.Now(),
		Name:   f.Name(),
		Status: false,
	}
	_, err := util.IsInterfaceDown(config.GlobalConfigInstance.VrrpNetInterface)
	if err != nil {
		metrics.FrontInterfaceCheckCounter.WithLabelValues("failed").Inc()
		sa.Extra = err.Error()
		return sa
	}
	sa.Status = true
	return sa

}
