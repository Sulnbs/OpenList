package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

// PluginDriverAdapter adapts plugin drivers to the main program's driver interface
type PluginDriverAdapter struct {
	pluginClient DriverPluginClient  // Use client interface
	driverName   string
	storage      *model.Storage
}

// NewPluginDriverAdapter creates a new adapter for a plugin driver
func NewPluginDriverAdapter(pluginClient DriverPluginClient, driverName string) *PluginDriverAdapter {
	return &PluginDriverAdapter{
		pluginClient: pluginClient,
		driverName:   driverName,
	}
}

// Config implements driver.Meta interface
func (a *PluginDriverAdapter) Config() driver.Config {
	// Convert plugin config to driver config
	// For now, return a placeholder - this would need to be implemented based on actual plugin config structure
	return driver.Config{
		Name:            a.driverName,
		LocalSort:       false,
		OnlyLinkMFile:   false,
		OnlyProxy:       false,
		NoCache:         false,
		NoUpload:        false,
		NeedMs:          false,
		DefaultRoot:     "/",
		CheckStatus:     false,
		Alert:           "",
		NoOverwriteUpload: false,
		ProxyRangeOption:  false,
		NoLinkURL:       false,
	}
}

// GetStorage implements driver.Meta interface
func (a *PluginDriverAdapter) GetStorage() *model.Storage {
	return a.storage
}

// SetStorage implements driver.Meta interface
func (a *PluginDriverAdapter) SetStorage(storage model.Storage) {
	a.storage = &storage
}

// GetAddition implements driver.Meta interface
func (a *PluginDriverAdapter) GetAddition() driver.Additional {
	// Return a generic additional configuration
	// This would need to be implemented based on actual plugin requirements
	return make(map[string]interface{})
}

// Init implements driver.Meta interface
func (a *PluginDriverAdapter) Init(ctx context.Context) error {
	// Convert model.Storage to map[string]interface{}
	storageData, err := structToMap(*a.storage)
	if err != nil {
		return fmt.Errorf("failed to convert storage to map: %w", err)
	}
	return a.pluginClient.InitDriver(a.driverName, storageData)
}

// Drop implements driver.Meta interface
func (a *PluginDriverAdapter) Drop(ctx context.Context) error {
	return a.pluginClient.DropDriver(a.driverName)
}

// List implements driver.Reader interface
func (a *PluginDriverAdapter) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	// Convert model.Obj and model.ListArgs to map[string]interface{}
	dirData, err := structToMap(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to convert dir to map: %w", err)
	}
	
	argsData, err := structToMap(args)
	if err != nil {
		return nil, fmt.Errorf("failed to convert args to map: %w", err)
	}
	
	respData, err := a.pluginClient.List(a.driverName, dirData, argsData)
	if err != nil {
		return nil, err
	}
	
	// Convert response back to []model.Obj
	var result []model.Obj
	err = json.Unmarshal(respData, &result)
	return result, err
}

// Link implements driver.Reader interface
func (a *PluginDriverAdapter) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	// Convert model.Obj and model.LinkArgs to map[string]interface{}
	fileData, err := structToMap(file)
	if err != nil {
		return nil, fmt.Errorf("failed to convert file to map: %w", err)
	}
	
	argsData, err := structToMap(args)
	if err != nil {
		return nil, fmt.Errorf("failed to convert args to map: %w", err)
	}
	
	respData, err := a.pluginClient.Link(a.driverName, fileData, argsData)
	if err != nil {
		return nil, err
	}
	
	// Convert response back to *model.Link
	var result model.Link
	err = json.Unmarshal(respData, &result)
	return &result, err
}

// structToMap converts any struct to map[string]interface{} using JSON
func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}