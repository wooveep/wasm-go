package config

import (
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/tidwall/gjson"
)

// PluginConfig is the ai-memory plugin configuration root.
type PluginConfig struct{}

func (c *PluginConfig) FromJson(json gjson.Result, log log.Log) {
}

func (c *PluginConfig) FromJsonWithGlobal(json gjson.Result, global PluginConfig, log log.Log) {
	*c = global
}

func (c PluginConfig) Validate() error {
	return nil
}

func (c *PluginConfig) Complete(log log.Log) error {
	return nil
}
