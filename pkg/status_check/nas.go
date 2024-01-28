package status_check

import (
	"net/http"
	"time"

	"split_brain_check/pkg/metrics"
)

var _ StatusInterface = (*NasImpl)(nil)

// nas服务检测
type NasImpl struct {
	Address string
}

func (n *NasImpl) Name() string {
	return "nas"
}

func (n *NasImpl) CheckStatus() StatusAction {
	metrics.NasCheckCounter.WithLabelValues("total").Inc()
	sa := StatusAction{
		Time:   time.Now(),
		Name:   n.Name(),
		Status: !nasDisable,
	}
	if nasDisable && nasErr != nil {
		sa.Extra = nasErr
	}
	return sa
}

var (
	nasDisable bool // false
	nasErr     error
)

func backGroundNasCheck(address string) {
	var count = 0
	for {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get(address)
		if err != nil || resp.StatusCode != http.StatusOK {
			nasErr = errors.New("check nas failed")
			count++
		} else {
			nasDisable = false
			nasErr = nil
			count = 0
			if resp.Body != nil {
				resp.Body.Close()
			}
		}
		if count >= 2 {
			nasDisable = true
			count = 0
		}
		time.Sleep(5 * time.Second)
	}

}
