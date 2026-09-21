// config 业务配置：configx 绑定 demo.* 段
package config

import "github.com/gospacex/configx"

// Biz 业务参数
type Biz struct {
	TimeoutMS int `mapstructure:"timeout_ms"`
}

// Trace 调用链开关（用户核心需求：开启后分层路由入链，经 logx 实现）
type Trace struct {
	Enabled  bool   `mapstructure:"enabled"`
	Exporter string `mapstructure:"exporter"` // stdout / jaeger / otlp
}

// Config 示例工程配置
type Config struct {
	Biz   Biz   `mapstructure:"biz"`
	Trace Trace `mapstructure:"trace"`
}

// FillDefaults 缺省（调用链默认关闭）
func (c *Config) FillDefaults() {
	if c.Trace.Exporter == "" {
		c.Trace.Exporter = "stdout"
	}
}

// Load 经 configx 读配置文件并绑定 demo 段
func Load(path string) (*Config, error) {
	c, err := configx.ReadConfig(configx.ConfigOptions{ConfigFile: path})
	if err != nil {
		return nil, err
	}
	out := &Config{}
	out.FillDefaults()
	if err := c.Unmarshal("demo", out); err != nil {
		return nil, err
	}
	return out, nil
}
