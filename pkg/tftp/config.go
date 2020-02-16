package tftp

import (
	"fmt"
	"net"
)

type Config struct {
	AdminRestAddress    string
	MainLogFileName     string
	RequestsLogFileName string
	LocalInterface      string
	ListenPort          uint16
	DataPayloadSize     uint16
	MaxSendTries        uint
	socketTimeoutSecs   uint
}

func (conf *Config) Init() (err error) {
	conf.AdminRestAddress = ":8069"
	conf.MainLogFileName = "tftpd.log"
	conf.RequestsLogFileName = "tftpd_requests.log"
	conf.LocalInterface = "0.0.0.0"
	conf.ListenPort = 69
	conf.DataPayloadSize = 512
	conf.MaxSendTries = 3
	conf.socketTimeoutSecs = 5
	return
}

func (conf *Config) DeInit() (err error) {
	return
}

func (conf *Config) ListenAddr() (*net.UDPAddr, error) {
	if a, e := resolveUDPAddr(conf.LocalInterface, conf.ListenPort); e != nil {
		return a, fmt.Errorf("Listen address: %w", e) // wrap error
	} else {
		return a, e
	}
}
