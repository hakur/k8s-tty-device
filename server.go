package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

const (
	resourceName              = "hakur/tty"
	serverSocketPath          = pluginapi.DevicePluginPath + "tty_device.sock"
	healthcheckIntervalSecond = 5
)

type TtyDevicePlugin struct {
	devs          []*pluginapi.Device
	socketFile    string
	grpcServer    *grpc.Server
	stopSig       chan interface{}
	devsReportSig chan bool
}

func NewTtyDevicePlugin() *TtyDevicePlugin {
	return &TtyDevicePlugin{
		socketFile:    serverSocketPath,
		stopSig:       make(chan interface{}),
		devsReportSig: make(chan bool),
		devs:          getDevices(),
	}
}

// cleanup : delete old data,such as socket file
func (t *TtyDevicePlugin) cleanup() error {
	logrus.Info("Run cleanup")
	if err := os.Remove(t.socketFile); err != nil && !os.IsNotExist(err) {
		logrus.Error("Run cleanup error when delete socket file ", t.socketFile, " -> ", err)
		return err
	}
	return nil
}

// Restart : restart server
func (t *TtyDevicePlugin) Restart() error {
	err := t.Stop()
	if err != nil {
		logrus.Error("Faild to stop server ", err)
		return err
	}
	err = t.Serve()
	if err != nil {
		logrus.Error("Error when start Serve", err)
		return err
	}
	return nil
}

// Start : init server and cleanup old data
func (t *TtyDevicePlugin) Start() error {
	// remove unix socket file on restart
	if err := t.cleanup(); err != nil {
		return err
	}
	// create grpc server
	sock, err := net.Listen("unix", t.socketFile)
	if err != nil {
		return errors.New("start unix socket listen failed " + err.Error())
	}
	t.grpcServer = grpc.NewServer([]grpc.ServerOption{}...)

	// register server(will call Allocate() by kubelet when create container)
	pluginapi.RegisterDevicePluginServer(t.grpcServer, t)

	// start grpc server listen
	go func() {
		err := t.grpcServer.Serve(sock)
		if err != nil {
			logrus.Error("grpc server exited", err)
		}
	}()

	logrus.Println("Socket path is ", t.socketFile)
	//try to connect grpc  server to test connection
	conn, err := dialUnixGrpc(t.socketFile, 5*time.Second)
	if err != nil {
		return errors.New("test connect to grpc server failed " + err.Error())
	}
	logrus.Info("grpc server connection is ok")
	conn.Close()

	go t.healthcheck()

	return nil
}

// Stop : stop server and cleanup data
func (t *TtyDevicePlugin) Stop() error {
	if t.grpcServer == nil {
		return nil
	}
	t.grpcServer.Stop()
	t.grpcServer = nil
	close(t.stopSig)
	close(t.devsReportSig)
	return t.cleanup()
}

// Resgister : inherit from pluginapi Interface , this register device plugin name to kubernetes
func (t *TtyDevicePlugin) Resgister(kubeletEndpoint string, resourceName string) error {
	conn, err := dialUnixGrpc(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	// register resource to kuberentes cluster
	client := pluginapi.NewRegistrationClient(conn)
	requestData := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(t.socketFile),
		ResourceName: resourceName,
	}
	_, err = client.Register(context.Background(), requestData)
	if err != nil {
		return err
	}
	return nil
}

// Allocate : inherit from pluginapi Interface , this is moust important section , this define which device will mount to container
func (t *TtyDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	dev := new(pluginapi.DeviceSpec)
	dev.HostPath = "/dev/tty99"
	dev.ContainerPath = "/dev/tty0"
	dev.Permissions = "rw"

	response.Devices = append(response.Devices, dev)

	return &response, nil
}

// ListAndWatch : inherit from pluginapi Interface , this tell kubernetes how many devices on this node and their health status
func (t *TtyDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	for {
		select {
		case <-t.stopSig:
			return nil
		case <-t.devsReportSig:
			//report unhealthy device to kubernetes
			s.Send(&pluginapi.ListAndWatchResponse{Devices: t.devs})
		}
	}
}

// unhealth : report unhealth device and try to fix it
func (t *TtyDevicePlugin) unhealth(dev *pluginapi.Device) {
	logrus.Warning("Device unhealth detected, device is /dev/", dev.ID)
	// try to recreate device

	if err := createTtyDevices(dev.ID); err != nil {
		logrus.Error("Failed to create TTY device ", "/dev/"+dev.ID, " ", err)
	}
}

// healthcheck : healthcheck interval for devices
func (t *TtyDevicePlugin) healthcheck() {
	for {
		devs := []*pluginapi.Device{}
		for _, dev := range t.devs {
			//test write to tty device
			devPath := "/dev/" + dev.ID
			f, err := os.Open(devPath)
			if err != nil {
				logrus.Warning("Device is not readable , device is ", devPath, " -> ", err)
				f.Close()
				t.unhealth(dev)
			}
			dev.Health = pluginapi.Healthy
			devs = append(devs, dev)
		}
		t.devsReportSig <- true
		time.Sleep(healthcheckIntervalSecond * time.Second)
	}
}

// Serve : start listen and serve
func (t *TtyDevicePlugin) Serve() (err error) {
	if err = t.Start(); err != nil {
		logrus.Error("Failed to start device plugin server ", err)
		return err
	}
	logrus.Info("Succesfully start server at ", t.socketFile)

	if err = t.Resgister(pluginapi.KubeletSocket, resourceName); err != nil {
		logrus.Error("Failed to register resource type  ", resourceName)
		return err
	}
	logrus.Info("Succesfully register resource type ", resourceName)
	return err
}
