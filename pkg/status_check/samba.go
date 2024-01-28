package status_check

import (
	"time"

	"split_brain_check/pkg/metrics"
)

var _ StatusInterface = (*SambaImpl)(nil)

type SambaImpl struct {
}

func (n *SambaImpl) Name() string {
	return "samba"
}

// 缓存PID列表
var sambaCachePidList = map[int]int{}

func (n *SambaImpl) CheckStatus() StatusAction {
	now := time.Now()
	defer func() {
		used := time.Since(now)
		logger.Infof("check %s used:%v", n.Name(), used)
	}()
	metrics.SambaCheckCounter.WithLabelValues("total").Inc()
	sa := StatusAction{
		Time:   time.Now(),
		Name:   n.Name(),
		Status: false,
	}
	for k, v := range sambaCachePidList {
		alived := checkProcessPid(v)
		if !alived {
			delete(sambaCachePidList, k)
			continue
		}
		sa.Status = true
		return sa
	}
	result := command.ExecBinBashCmd(5*time.Second, `pgrep smbd`) // OpenEuler和Ubuntu一样
	if result.HasError() {
		metrics.SambaCheckCounter.WithLabelValues("failed").Inc()
		sa.Extra = result.Error()
		return sa
	}
	if len(result.StdOutput) == 0 {
		metrics.SambaCheckCounter.WithLabelValues("failed").Inc()
		sa.Extra = errors.New("has no smbd process")
		return sa
	}
	split, err := bytesToIntSlice(string(result.StdOutput), "\n")
	logger.Infof("all smbd pids:%v message:%v", split, err)
	for i := range split {
		sambaCachePidList[i] = split[i]
	}
	sa.Status = true
	return sa
}
