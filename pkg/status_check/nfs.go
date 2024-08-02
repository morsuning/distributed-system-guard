package status_check

import (
	"time"

	"system-usability-detection/pkg/metrics"
)

var _ StatusInterface = (*NFSImpl)(nil)

// NFSImpl nfs服务检测
type NFSImpl struct {
}

func (n *NFSImpl) Name() string {
	return "nfs"
}

// 缓存PID列表
var nfsCachePidList = map[int]int{}

func (n *NFSImpl) CheckStatus() StatusAction {
	now := time.Now()
	defer func() {
		used := time.Since(now)
		logger.Infof("check %s used:%v", n.Name(), used)
	}()
	metrics.NfsCheckCounter.WithLabelValues("total").Inc()
	sa := StatusAction{
		Time:   time.Now(),
		Name:   n.Name(),
		Status: false,
	}
	for k, v := range nfsCachePidList {
		alived := checkProcessPid(v)
		if !alived {
			delete(nfsCachePidList, k)
			continue
		}
		sa.Status = true
		return sa
	}
	result := command.ExecBinBashCmd(5*time.Second, `pgrep nfsd`)
	if result.HasError() {
		metrics.NfsCheckCounter.WithLabelValues("failed").Inc()
		sa.Extra = result.Error()
		return sa
	}
	if len(result.StdOutput) == 0 {
		metrics.NfsCheckCounter.WithLabelValues("failed").Inc()
		sa.Extra = errors.New("has no nfsd process")
		return sa
	}
	split, err := bytesToIntSlice(string(result.StdOutput), "\n")
	logger.Infof("all nfsd pids:%v message:%v", split, err)
	for i := range split {
		nfsCachePidList[i] = split[i]
	}
	sa.Status = true
	return sa
}
