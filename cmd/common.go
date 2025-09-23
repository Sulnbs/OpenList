package cmd

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/OpenListTeam/OpenList/v4/internal/bootstrap"
	"github.com/OpenListTeam/OpenList/v4/internal/bootstrap/data"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/plugin"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	log "github.com/sirupsen/logrus"
)

func Init() {
	bootstrap.InitConfig()
	bootstrap.Log()
	
	// Load plugins after config is initialized
	loadPlugins()
	
	bootstrap.InitDB()
	data.InitData()
	bootstrap.InitStreamLimit()
	bootstrap.InitIndex()
	bootstrap.InitUpgradePatch()
}

// loadPlugins initializes and loads driver plugins
func loadPlugins() {
	manager := plugin.GetManager()
	
	// Try to load plugins from standard locations
	pluginDirs := []string{
		"./plugins",
		"./data/plugins", 
		"/usr/local/share/openlist/plugins",
		"/opt/openlist/plugins",
	}
	
	for _, dir := range pluginDirs {
		if err := manager.LoadPluginsFromDir(dir); err != nil {
			log.Debugf("Failed to load plugins from %s: %v", dir, err)
		}
	}
}

func Release() {
	// Shutdown plugins before closing database
	plugin.GetManager().Shutdown()
	db.Close()
}

var pid = -1
var pidFile string

func initDaemon() {
	ex, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exPath := filepath.Dir(ex)
	_ = os.MkdirAll(filepath.Join(exPath, "daemon"), 0700)
	pidFile = filepath.Join(exPath, "daemon/pid")
	if utils.Exists(pidFile) {
		bytes, err := os.ReadFile(pidFile)
		if err != nil {
			log.Fatal("failed to read pid file", err)
		}
		id, err := strconv.Atoi(string(bytes))
		if err != nil {
			log.Fatal("failed to parse pid data", err)
		}
		pid = id
	}
}
