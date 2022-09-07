package pluginmanagerbaselib

import (
	"fmt"
	"github.com/hashicorp/go-plugin"
	"os"
	"sync"
)

var (
	lock = &sync.Mutex{}
	// pluginTypeMap 插件类型map
	pluginTypeMap = make(map[string]plugin.PluginSet)
	// pluginMap 插件列表集合, key为插件类型名称, val为插件map
	pluginInfoMap = make(map[string][]*PluginInfo)
)

// AddPluginTypeInterface 添加插件类型接口, typeName 类型名称, pluginInterfaceName 插件接口名称, pluginInterface 插件接口
func AddPluginTypeInterface(typeName, pluginInterfaceName string, pluginInterface plugin.Plugin) {
	lock.Lock()
	defer lock.Unlock()

	pluginSet, ok := pluginTypeMap[typeName]
	if !ok {
		pluginSet = make(map[string]plugin.Plugin)
		pluginTypeMap[typeName] = pluginSet
	}

	pluginSet[pluginInterfaceName] = pluginInterface
}

// AddPlugin 添加一个插件信息
func AddPlugin(pluginTypeName string, pluginInfo *PluginInfo) error {
	lock.Lock()
	defer lock.Unlock()

	if pluginInfo == nil {
		return fmt.Errorf("插件信息不能为空")
	}

	if pluginInfo.Name == "" {
		return fmt.Errorf("插件名称不能为空")
	}

	if pluginInfo.PluginFilePath == "" {
		return fmt.Errorf("插件文件路径不能为空")
	}

	if stat, err := os.Stat(pluginInfo.PluginFilePath); err != nil || stat.IsDir() {
		return fmt.Errorf("插件文件[%s]不存在", pluginInfo.PluginFilePath)
	}

	if pluginMap, ok := pluginTypeMap[pluginTypeName]; !ok {
		return fmt.Errorf("未识别的插件类别: %s", pluginTypeName)
	} else if _, ok = pluginMap[pluginInfo.Name]; !ok {
		return fmt.Errorf("未被注册的插件名称: %s", pluginInfo.Name)
	}

	defer pluginInfo.start()
	pluginList, ok := pluginInfoMap[pluginTypeName]
	if !ok {
		pluginInfoMap[pluginTypeName] = []*PluginInfo{pluginInfo}
		return nil
	}

	pluginFlag := pluginInfo.Id + pluginInfo.Name
	for i := range pluginList {
		p := pluginList[i]
		if p.Id+pluginInfo.Name == pluginFlag {
			return fmt.Errorf("插件[%s]已存在", pluginInfo.Id)
		}
	}

	pluginList = append(pluginList, pluginInfo)
	return nil
}
