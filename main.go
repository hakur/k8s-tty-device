package main

import (
	"github.com/sirupsen/logrus"
	fsnotify "gopkg.in/fsnotify.v1"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

func main() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	devicePlugin := NewTtyDevicePlugin()
	err := devicePlugin.Serve()
	if err != nil {
		logrus.Fatal(err)
	}
	// create a kubelet unix socket file watcher , when file created , plugin shuold have a restart
	fswatcher, err := newFileWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		logrus.Fatal("Failed create fsWatcher for watch file ", err)
	}
	defer fswatcher.Close()
	logrus.Info("Successfully create fsWatcher")

	for {
		select {
		case event := <-fswatcher.Events:
			//reveived kubelet socket created, we need to restart our server for reconnect kubelet device plugin unix socket
			if event.Name == pluginapi.KubeletSocket && (event.Op&fsnotify.Create == fsnotify.Create) {
				devicePlugin.Restart()
			}
		case err := <-fswatcher.Errors:
			logrus.Error("fsnotify got an error: %s", err)
		}
	}
}
