package status_check

import (
	"system-usability-detection/internal/util"
	"system-usability-detection/pkg/metrics"
	"time"
)

var _ StatusInterface = (*OSSImpl)(nil)

var cacheOSSPid = map[int]int{}

type OSSImpl struct {
}

func (u *OSSImpl) Name() string {
	return "OSS"
}
func (u *OSSImpl) CheckStatus() StatusAction {
	now := time.Now()
	defer func() {
		used := time.Since(now)
		util.Logger.Info("check %s used:%v", u.Name(), used)
	}()
	metrics.ServiceCheckCounter.WithLabelValues("total").Inc()
	sa := StatusAction{
		Time:   time.Now(),
		Name:   u.Name(),
		Status: false,
	}

	for k, v := range cacheOSSPid {
		alived := util.CheckProcessPid(v)
		if !alived {
			util.Logger.Info("process is not alived,pid:%d", v)
			delete(cacheOSSPid, k)
			continue
		}
		sa.Status = true
		return sa
	}
	// check process
	sa.Status = true
	return sa
}
