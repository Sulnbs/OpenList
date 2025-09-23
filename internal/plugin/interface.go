package plugin

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

// DriverPluginImpl implements the plugin.Plugin interface for the client side
type DriverPluginImpl struct {
	plugin.Plugin
}

func (p *DriverPluginImpl) Server(*plugin.MuxBroker) (interface{}, error) {
	return nil, nil
}

func (p *DriverPluginImpl) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &DriverPluginRPC{client: c}, nil
}

// DriverPluginClient interface for main program to call plugins
type DriverPluginClient interface {
	GetInfo() (PluginInfo, error)
	GetDrivers() ([]string, error)
	GetDriverConfig(name string) (map[string]interface{}, error)
	InitDriver(driverName string, storageData map[string]interface{}) error
	DropDriver(driverName string) error
	List(driverName string, dirData map[string]interface{}, argsData map[string]interface{}) ([]byte, error)
	Link(driverName string, fileData map[string]interface{}, argsData map[string]interface{}) ([]byte, error)
}

// PluginInfo contains plugin metadata
type PluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// DriverPluginRPC implements DriverPluginClient interface
type DriverPluginRPC struct {
	client *rpc.Client
}

func (g *DriverPluginRPC) GetInfo() (PluginInfo, error) {
	var resp PluginInfo
	err := g.client.Call("Plugin.GetInfo", new(interface{}), &resp)
	return resp, err
}

func (g *DriverPluginRPC) GetDrivers() ([]string, error) {
	var resp []string
	err := g.client.Call("Plugin.GetDrivers", new(interface{}), &resp)
	return resp, err
}

func (g *DriverPluginRPC) GetDriverConfig(name string) (map[string]interface{}, error) {
	var resp map[string]interface{}
	err := g.client.Call("Plugin.GetDriverConfig", name, &resp)
	return resp, err
}

func (g *DriverPluginRPC) InitDriver(driverName string, storageData map[string]interface{}) error {
	args := map[string]interface{}{
		"driver_name":  driverName,
		"storage_data": storageData,
	}
	return g.client.Call("Plugin.InitDriver", args, new(interface{}))
}

func (g *DriverPluginRPC) DropDriver(driverName string) error {
	return g.client.Call("Plugin.DropDriver", driverName, new(interface{}))
}

func (g *DriverPluginRPC) List(driverName string, dirData map[string]interface{}, argsData map[string]interface{}) ([]byte, error) {
	callArgs := map[string]interface{}{
		"driver_name": driverName,
		"dir_data":    dirData,
		"args_data":   argsData,
	}
	
	var respData []byte
	err := g.client.Call("Plugin.List", callArgs, &respData)
	return respData, err
}

func (g *DriverPluginRPC) Link(driverName string, fileData map[string]interface{}, argsData map[string]interface{}) ([]byte, error) {
	callArgs := map[string]interface{}{
		"driver_name": driverName,
		"file_data":   fileData,
		"args_data":   argsData,
	}
	
	var respData []byte
	err := g.client.Call("Plugin.Link", callArgs, &respData)
	return respData, err
}