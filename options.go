package dbconnpool

import (
	"context"
	"net"
	"time"
)

type Options struct {
	Host        string
	Port        string
	User        string
	Password    string
	Database    string
	Network     string
	DialTimeout time.Duration
	//Todo TSL CONFIG and timeouts (read, write) and pools
	Diler func(ctx context.Context, netwrok string, addr string) (net.Conn, error)
}

func (opt *Options) init() {
	if opt.Host == "" {
		opt.Host = "localhost"
	}

	if opt.Port == "" {
		opt.Port = "5432"
	}

	if opt.Network == "" {
		opt.Network = "tcp"
	}

	if opt.DialTimeout == 0 {
		opt.DialTimeout = 5 * time.Second
	}

	if opt.Diler == nil { //Diler is a filed that can hold a function with that exact signature
		opt.Diler = func(ctx context.Context, netwrok string, addr string) (net.Conn, error) {
			netDialer := &net.Dialer{
				Timeout:   opt.DialTimeout,
				KeepAlive: 5 * time.Minute,
			}
			return netDialer.DialContext(ctx, netwrok, addr)
		}
	}

}
