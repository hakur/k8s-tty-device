package main

import (
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

// get device list
func getDevices() (devs []*pluginapi.Device) {
	dev := &pluginapi.Device{ID: "tty99", Health: pluginapi.Healthy}
	devs = append(devs, dev)
	return devs
}

// connectDeviceSock  connect to unix sock that created by kubelet for device health check , etc...
func dialUnixGrpc(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial("unix://"+unixSocketPath, grpc.WithTimeout(timeout),
		grpc.WithBlock(),
		grpc.WithInsecure(),
	)

	if err != nil {
		return nil, err
	}
	return conn, err
}

// mknod -m 0620 /dev/tty99 c 4 0
func createTtyDevices(ttyName string) error {
	cmd := exec.Command("mknod", []string{"-m", "0620", "/dev/" + ttyName, "c", "4", "0"}...)
	logrus.Info("Creeate tty device /dev/", ttyName, ", command is [ ", strings.Join(cmd.Args, " "), " ]")
	err := cmd.Run()
	buf, _ := cmd.Output()
	if err != nil {
		return errors.New(err.Error() + " : " + string(buf))
	}

	cmd = exec.Command("chown", []string{"root:tty", "/dev/" + ttyName}...)
	logrus.Info("Set permission tty device /dev/", ttyName, ", command is [ ", strings.Join(cmd.Args, " "), " ]")
	err = cmd.Run()
	buf, _ = cmd.Output()
	if err != nil {
		return errors.New(err.Error() + " : " + string(buf))
	}
	return nil
}
