package op

import (
	"reflect"
	"strings"
	"sync"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/pkg/errors"
)

type DriverConstructor func() driver.Driver

var driverMap = map[string]DriverConstructor{}
var driverInfoMap = map[string]driver.Info{}
var driverMutex sync.RWMutex

func RegisterDriver(driver DriverConstructor) {
	driverMutex.Lock()
	defer driverMutex.Unlock()
	
	// log.Infof("register driver: [%s]", config.Name)
	tempDriver := driver()
	tempConfig := tempDriver.Config()
	registerDriverItems(tempConfig, tempDriver.GetAddition())
	driverMap[tempConfig.Name] = driver
}

// UnregisterDriver removes a driver from registry
func UnregisterDriver(name string) {
	driverMutex.Lock()
	defer driverMutex.Unlock()
	
	delete(driverMap, name)
	delete(driverInfoMap, name)
}

// IsDriverRegistered checks if a driver is already registered
func IsDriverRegistered(name string) bool {
	driverMutex.RLock()
	defer driverMutex.RUnlock()
	
	_, exists := driverMap[name]
	return exists
}

func GetDriver(name string) (DriverConstructor, error) {
	driverMutex.RLock()
	defer driverMutex.RUnlock()
	
	n, ok := driverMap[name]
	if !ok {
		return nil, errors.Errorf("no driver named: %s", name)
	}
	return n, nil
}

func GetDriverNames() []string {
	driverMutex.RLock()
	defer driverMutex.RUnlock()
	
	var driverNames []string
	for k := range driverInfoMap {
		driverNames = append(driverNames, k)
	}
	return driverNames
}

func GetDriverInfoMap() map[string]driver.Info {
	driverMutex.RLock()
	defer driverMutex.RUnlock()
	
	// Return a copy to prevent race conditions
	result := make(map[string]driver.Info)
	for k, v := range driverInfoMap {
		result[k] = v
	}
	return result
}

func registerDriverItems(config driver.Config, addition driver.Additional) {
	defer func() {
		driverMutex.Unlock()
	}()
	driverMutex.Lock()
	// log.Debugf("addition of %s: %+v", config.Name, addition)
	tAddition := reflect.TypeOf(addition)
	for tAddition.Kind() == reflect.Pointer {
		tAddition = tAddition.Elem()
	}
	mainItems := getMainItems(config)
	
	var additionalItems []driver.Item
	// Handle map type for plugin drivers
	if tAddition.Kind() == reflect.Map {
		additionalItems = []driver.Item{} // Skip additional items for plugin drivers
	} else {
		additionalItems = getAdditionalItems(tAddition, config.DefaultRoot)
	}
	
	driverInfoMap[config.Name] = driver.Info{
		Common:     mainItems,
		Additional: additionalItems,
		Config:     config,
	}
}

func getMainItems(config driver.Config) []driver.Item {
	items := []driver.Item{{
		Name:     "mount_path",
		Type:     conf.TypeString,
		Required: true,
		Help:     "The path you want to mount to, it is unique and cannot be repeated",
	}, {
		Name: "order",
		Type: conf.TypeNumber,
		Help: "use to sort",
	}, {
		Name: "remark",
		Type: conf.TypeText,
	}}
	if !config.NoCache {
		items = append(items, driver.Item{
			Name:     "cache_expiration",
			Type:     conf.TypeNumber,
			Default:  "30",
			Required: true,
			Help:     "The cache expiration time for this storage",
		})
	}
	if config.MustProxy() {
		items = append(items, driver.Item{
			Name:     "webdav_policy",
			Type:     conf.TypeSelect,
			Default:  "native_proxy",
			Options:  "use_proxy_url,native_proxy",
			Required: true,
		})
	} else {
		items = append(items, []driver.Item{{
			Name: "web_proxy",
			Type: conf.TypeBool,
		}, {
			Name:     "webdav_policy",
			Type:     conf.TypeSelect,
			Options:  "302_redirect,use_proxy_url,native_proxy",
			Default:  "302_redirect",
			Required: true,
		},
		}...)
		if config.ProxyRangeOption {
			item := driver.Item{
				Name: "proxy_range",
				Type: conf.TypeBool,
				Help: "Need to enable proxy",
			}
			if config.Name == "139Yun" {
				item.Default = "true"
			}
			items = append(items, item)
		}
	}
	items = append(items, driver.Item{
		Name: "down_proxy_url",
		Type: conf.TypeText,
	})
	items = append(items, driver.Item{
		Name:    "disable_proxy_sign",
		Type:    conf.TypeBool,
		Default: "false",
		Help:    "Disable sign for Download proxy URL",
	})
	if config.LocalSort {
		items = append(items, []driver.Item{{
			Name:    "order_by",
			Type:    conf.TypeSelect,
			Options: "name,size,modified",
		}, {
			Name:    "order_direction",
			Type:    conf.TypeSelect,
			Options: "asc,desc",
		}}...)
	}
	items = append(items, driver.Item{
		Name:    "extract_folder",
		Type:    conf.TypeSelect,
		Options: "front,back",
	})
	items = append(items, driver.Item{
		Name:     "disable_index",
		Type:     conf.TypeBool,
		Default:  "false",
		Required: true,
	})
	items = append(items, driver.Item{
		Name:     "enable_sign",
		Type:     conf.TypeBool,
		Default:  "false",
		Required: true,
	})
	return items
}
func getAdditionalItems(t reflect.Type, defaultRoot string) []driver.Item {
	var items []driver.Item
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Struct {
			items = append(items, getAdditionalItems(field.Type, defaultRoot)...)
			continue
		}
		tag := field.Tag
		ignore, ok1 := tag.Lookup("ignore")
		name, ok2 := tag.Lookup("json")
		if (ok1 && ignore == "true") || !ok2 {
			continue
		}
		item := driver.Item{
			Name:     name,
			Type:     strings.ToLower(field.Type.Name()),
			Default:  tag.Get("default"),
			Options:  tag.Get("options"),
			Required: tag.Get("required") == "true",
			Help:     tag.Get("help"),
		}
		if tag.Get("type") != "" {
			item.Type = tag.Get("type")
		}
		if item.Name == "root_folder_id" || item.Name == "root_folder_path" {
			if item.Default == "" {
				item.Default = defaultRoot
			}
			item.Required = item.Default != ""
		}
		// set default type to string
		if item.Type == "" {
			item.Type = "string"
		}
		items = append(items, item)
	}
	return items
}
