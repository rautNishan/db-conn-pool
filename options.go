package dbconnpool

type Options struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	//Todo TSL CONFIG and timeouts (Dial timeout, read, write) and pools
}

func (opt *Options) init() {
	if opt.Host == "" {
		opt.Host = "localhost"
	}

	if opt.Port == "" {
		opt.Port = "5432"
	}
}
