package util

import (
	"errors"
	"net"
)

// IsInterfaceDown 判断网卡是否是down状态
func IsInterfaceDown(name string) (*net.Interface, error) {
	face, err := net.InterfaceByName(name)
	if err != nil {
		return nil, errors.New("get net interface failed:" + err.Error() + "or net interface is down")
	}
	if face == nil {
		return nil, errors.New("get net interface failed:" + " interface  " + name + " not found")
	}
	if face.Flags&net.FlagUp == 0 {
		return nil, errors.New("net interface is down")
	}
	if face.Flags&net.FlagRunning == 0 {
		return nil, errors.New("net interface is not running")
	}
	return face, nil
}
