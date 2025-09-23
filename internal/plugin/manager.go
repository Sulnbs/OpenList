package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-plugin"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/sirupsen/logrus"
)

// Manager manages storage driver plugins
type Manager struct {
	plugins map[string]*PluginInstance
	mu      sync.RWMutex
}

// PluginInstance represents a loaded plugin
type PluginInstance struct {
	Name     string
	Path     string
	Client   *plugin.Client
	Drivers  []string
	Config   map[string]interface{}
}

var globalManager *Manager
var once sync.Once

// GetManager returns the global plugin manager
func GetManager() *Manager {
	once.Do(func() {
		globalManager = &Manager{
			plugins: make(map[string]*PluginInstance),
		}
	})
	return globalManager
}

// LoadPluginsFromDir scans directory for plugin files and loads them
func (m *Manager) LoadPluginsFromDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.Debugf("Plugin directory %s does not exist, skipping", dir)
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if filepath.Ext(name) == ".exe" || filepath.Ext(name) == "" {
			pluginPath := filepath.Join(dir, name)
			if err := m.LoadPlugin(pluginPath); err != nil {
				logrus.Errorf("Failed to load plugin %s: %v", pluginPath, err)
			}
		}
	}

	return nil
}

// LoadPlugin loads a single plugin
func (m *Manager) LoadPlugin(pluginPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if plugin is executable
	if _, err := os.Stat(pluginPath); err != nil {
		return fmt.Errorf("plugin file not found: %w", err)
	}

	// Create plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "OPENLIST_PLUGIN",
			MagicCookieValue: "driver-plugin",
		},
		Plugins: map[string]plugin.Plugin{
			"driver-plugin": &DriverPluginImpl{},
		},
		Cmd:     exec.Command(pluginPath),
		Managed: true, // Enable managed mode for background execution
	})

	// Connect to plugin
	logrus.Debugf("Connecting to plugin RPC client...")
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to connect to plugin: %w", err)
	}
	logrus.Debugf("RPC client connected successfully")

	// Get plugin interface
	logrus.Debugf("Dispensing plugin interface...")
	raw, err := rpcClient.Dispense("driver-plugin")
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to get plugin interface: %w", err)
	}
	logrus.Debugf("Plugin interface dispensed successfully")

	driverPlugin := raw.(DriverPluginClient)  // Use client interface
	
	// Get plugin info
	logrus.Debugf("Getting plugin info...")
	info, err := driverPlugin.GetInfo()
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to get plugin info: %w", err)
	}
	logrus.Debugf("Plugin info received: %+v", info)

	// Check if plugin is already loaded
	if _, exists := m.plugins[info.Name]; exists {
		client.Kill()
		logrus.Debugf("Plugin %s already loaded, skipping", info.Name)
		return nil
	}

	logrus.Debugf("Getting drivers from plugin %s...", info.Name)
	// Get available drivers
	drivers, err := driverPlugin.GetDrivers()
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to get plugin drivers: %w", err)
	}
	logrus.Debugf("Plugin %s provides %d drivers: %v", info.Name, len(drivers), drivers)

	// Register plugin drivers with main program
	for _, driverName := range drivers {
		logrus.Debugf("Registering driver %s from plugin %s", driverName, info.Name)
		m.registerPluginDriver(driverName, info.Name, driverPlugin)
	}

	// Store plugin instance
	pluginInstance := &PluginInstance{
		Name:    info.Name,
		Path:    pluginPath,
		Client:  client,
		Drivers: drivers,
		Config:  make(map[string]interface{}),
	}

	m.plugins[info.Name] = pluginInstance
	
	logrus.Infof("Loaded plugin %s with drivers: %v", info.Name, drivers)
	return nil
}

// registerPluginDriver registers a plugin driver with the main driver system
func (m *Manager) registerPluginDriver(driverName, pluginName string, pluginClient DriverPluginClient) {
	// Check for driver name conflicts
	if op.IsDriverRegistered(driverName) {
		logrus.Warnf("Driver %s already registered, skipping plugin %s", driverName, pluginName)
		return
	}

	// Create driver constructor that uses plugin client interface
	constructor := func() driver.Driver {
		return NewPluginDriverAdapter(pluginClient, driverName)
	}

	// Register with main driver system
	op.RegisterDriver(constructor)
	logrus.Infof("Registered plugin driver: %s from plugin: %s", driverName, pluginName)
}

// UnloadPlugin unloads a specific plugin and its drivers
func (m *Manager) UnloadPlugin(pluginName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	// Unregister all drivers from this plugin
	for _, driverName := range instance.Drivers {
		op.UnregisterDriver(driverName)
		logrus.Infof("Unregistered driver: %s from plugin: %s", driverName, pluginName)
	}

	// Kill plugin process
	instance.Client.Kill()

	// Remove from manager
	delete(m.plugins, pluginName)

	logrus.Infof("Unloaded plugin: %s", pluginName)
	return nil
}

// ReloadPlugin reloads a specific plugin
func (m *Manager) ReloadPlugin(pluginName string) error {
	instance, exists := m.plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	pluginPath := instance.Path

	// Unload existing plugin
	if err := m.UnloadPlugin(pluginName); err != nil {
		return fmt.Errorf("failed to unload plugin %s: %w", pluginName, err)
	}

	// Reload plugin
	if err := m.LoadPlugin(pluginPath); err != nil {
		return fmt.Errorf("failed to reload plugin %s: %w", pluginName, err)
	}

	return nil
}

// GetPluginClient returns the client interface for a specific plugin
func (m *Manager) GetPluginClient(pluginName string) (DriverPluginClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.plugins[pluginName]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}

	// Get RPC client from plugin client
	rpcClient, err := instance.Client.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to get RPC client: %w", err)
	}

	// Get plugin interface
	raw, err := rpcClient.Dispense("driver-plugin")
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin interface: %w", err)
	}

	return raw.(DriverPluginClient), nil
}

// GetLoadedPlugins returns list of currently loaded plugins
func (m *Manager) GetLoadedPlugins() map[string]*PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*PluginInstance)
	for k, v := range m.plugins {
		result[k] = v
	}
	return result
}

// GetPluginDrivers returns drivers provided by a specific plugin
func (m *Manager) GetPluginDrivers(pluginName string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.plugins[pluginName]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}

	return instance.Drivers, nil
}

// RescanPlugins rescans all plugin directories for new plugins
func (m *Manager) RescanPlugins() error {
	pluginDirs := []string{
		"./plugins",
		"./data/plugins",
		"/usr/local/share/openlist/plugins",
		"/opt/openlist/plugins",
	}

	for _, dir := range pluginDirs {
		if err := m.LoadPluginsFromDir(dir); err != nil {
			logrus.Debugf("Failed to rescan plugins from %s: %v", dir, err)
		}
	}

	return nil
}

// Shutdown stops all loaded plugins
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, instance := range m.plugins {
		logrus.Infof("Shutting down plugin: %s", name)
		instance.Client.Kill()
	}
	
	m.plugins = make(map[string]*PluginInstance)
}