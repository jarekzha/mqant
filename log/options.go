/**
 * description: 选项
 * author: jarekzha@gmail.com
 * date: Aug 24, 2022
 */
package log

type Options struct {
	Debug       bool
	ProcessID   string
	LogDir      string
	LogFileName func(logdir, prefix, processID, suffix string) string
	LogSetting  map[string]interface{}
}

type Option func(cc *Options) Option

func WithDebug(v bool) Option {
	return func(cc *Options) Option {
		previous := cc.Debug
		cc.Debug = v
		return WithDebug(previous)
	}
}

func WithProcessID(v string) Option {
	return func(cc *Options) Option {
		previous := cc.ProcessID
		cc.ProcessID = v
		return WithProcessID(previous)
	}
}

func WithLogDir(v string) Option {
	return func(cc *Options) Option {
		previous := cc.LogDir
		cc.LogDir = v
		return WithLogDir(previous)
	}
}

func WithLogFileName(v func(logdir, prefix, processID, suffix string) string) Option {
	return func(cc *Options) Option {
		previous := cc.LogFileName
		cc.LogFileName = v
		return WithLogFileName(previous)
	}
}

func WithLogSetting(v map[string]interface{}) Option {
	return func(cc *Options) Option {
		previous := cc.LogSetting
		cc.LogSetting = v
		return WithLogSetting(previous)
	}
}

func NewOptions(opts ...Option) *Options {
	cc := &Options{
		Debug: false,
	}

	for _, opt := range opts {
		_ = opt(cc)
	}
	return cc
}
