package main

import (
	"fmt"
	"log"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/plugin"
)

func main() {
	fmt.Println("测试Manager带调试信息...")
	
	manager := plugin.GetManager()
	
	pluginPath := "../../openlist-storage-driver-plugins/plugins/open.exe"
	
	done := make(chan error, 1)
	go func() {
		done <- manager.LoadPlugin(pluginPath)
	}()
	
	select {
	case err := <-done:
		if err != nil {
			log.Fatalf("加载插件失败: %v", err)
		}
		fmt.Println("插件加载成功!")
		manager.Shutdown()
		
	case <-time.After(15 * time.Second):
		log.Println("超时，强制退出")
		return
	}
}