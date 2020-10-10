package upnp

import (
	"errors"
	"fmt"
	"sync"
)

/*
 * 得到网关
 */

type PortMapping struct {
	localPort   int
	remotePort  int
	protocol    string
	description string
}

//对所有的端口进行管理
type MappingPortStruct struct {
	lock         *sync.Mutex
	mappingPorts map[string][]PortMapping
}

//添加一个端口映射记录
//只对映射进行管理
func (this *MappingPortStruct) addMapping(localPort, remotePort int, protocol string, description string) {

	this.lock.Lock()
	defer this.lock.Unlock()
	if this.mappingPorts == nil {
		this.mappingPorts = map[string][]PortMapping{}
	}
	portMappings := this.mappingPorts[protocol]
	for i := 0; i < len(portMappings); i++ {
		portMapping := portMappings[i]
		if portMapping.localPort == localPort && portMapping.remotePort == remotePort {
			return
		}
	}

	portMapping := PortMapping{localPort: localPort, remotePort: remotePort, protocol: protocol, description: description}
	fmt.Println("add port mapping", portMapping)
	this.mappingPorts[protocol] = append(portMappings, portMapping)
}

//删除一个映射记录
//只对映射进行管理
func (this *MappingPortStruct) delMapping(remotePort int, protocol string) {
	this.lock.Lock()
	defer this.lock.Unlock()
	if this.mappingPorts == nil {
		return
	}
	tmp := []PortMapping{}
	mappings := this.mappingPorts[protocol]
	for i := 0; i < len(mappings); i++ {
		if mappings[i].remotePort == remotePort {
			//要删除的映射
			continue
		}
		tmp = append(tmp, mappings[i])
	}
	this.mappingPorts[protocol] = tmp
}
func (this *MappingPortStruct) GetAllMapping() map[string][]PortMapping {
	return this.mappingPorts
}

type Upnp struct {
	Active             bool              //这个upnp协议是否可用
	LocalHost          string            //本机ip地址
	GatewayInsideIP    string            //局域网网关ip
	GatewayOutsideIP   string            //网关公网ip
	OutsideMappingPort map[string]int    //映射外部端口
	InsideMappingPort  map[string]int    //映射本机端口
	Gateway            *Gateway          //网关信息
	CtrlUrl            string            //控制请求url
	MappingPort        MappingPortStruct //已经添加了的映射 {"TCP":[1990],"UDP":[1991]}
}

//得到本地联网的ip地址
//得到局域网网关ip
func (this *Upnp) SearchGateway() (err error) {
	defer func(err error) {
		if errTemp := recover(); errTemp != nil {
			fmt.Println("upnp模块报错了", errTemp)
			err = errTemp.(error)
		}
	}(err)

	if this.LocalHost == "" {
		this.MappingPort = MappingPortStruct{
			lock: new(sync.Mutex),
			// mappingPorts: map[string][][]int{},
		}
		this.LocalHost = GetLocalIntenetIp()
	}
	searchGateway := SearchGateway{upnp: this}
	if searchGateway.Send() {
		return nil
	}
	return errors.New("未发现网关设备")
}

func (this *Upnp) deviceStatus() {

}

//查看设备描述，得到控制请求url
func (this *Upnp) deviceDesc() (err error) {
	if this.GatewayInsideIP == "" {
		if err := this.SearchGateway(); err != nil {
			return err
		}
	}
	device := DeviceDesc{upnp: this}
	device.Send()
	this.Active = true
	fmt.Println("获得控制请求url:", this.CtrlUrl)
	return
}

//查看公网ip地址
func (this *Upnp) ExternalIPAddr() (err error) {
	if this.CtrlUrl == "" {
		if err := this.deviceDesc(); err != nil {
			return err
		}
	}
	eia := ExternalIPAddress{upnp: this}
	eia.Send()
	fmt.Println("获得公网ip地址为：", this.GatewayOutsideIP)
	return nil
}

//添加一个端口映射
func (this *Upnp) AddPortMapping(localPort, remotePort int, protocol, description string) (err error) {
	defer func(err error) {
		if errTemp := recover(); errTemp != nil {
			fmt.Println("upnp模块报错了", errTemp)
			err = errTemp.(error)
		}
	}(err)
	if this.GatewayOutsideIP == "" {
		if err := this.ExternalIPAddr(); err != nil {
			return err
		}
	}
	this.DelPortMapping(remotePort, protocol)
	addPort := AddPortMapping{upnp: this}
	if issuccess := addPort.Send(localPort, remotePort, protocol, description); issuccess {
		this.MappingPort.addMapping(localPort, remotePort, protocol, description)
		fmt.Println("添加一个端口映射：protocol:", protocol, "local:", localPort, "remote:", remotePort)
		return nil
	} else {
		this.Active = false
		fmt.Println("添加一个端口映射失败")
		return errors.New("添加一个端口映射失败")
	}
}

func (this *Upnp) DelPortMapping(remotePort int, protocol string) bool {
	delMapping := DelPortMapping{upnp: this}
	issuccess := delMapping.Send(remotePort, protocol)
	if issuccess {
		this.MappingPort.delMapping(remotePort, protocol)
		fmt.Println("删除了一个端口映射： remote:", remotePort)
	}
	return issuccess
}

//回收端口
func (this *Upnp) Reclaim() {
	mappings := this.MappingPort.GetAllMapping()
	tcpMapping, ok := mappings["TCP"]
	if ok {
		for i := 0; i < len(tcpMapping); i++ {
			this.DelPortMapping(tcpMapping[i].remotePort, "TCP")
		}
	}
	udpMapping, ok := mappings["UDP"]
	if ok {
		for i := 0; i < len(udpMapping); i++ {
			this.DelPortMapping(udpMapping[i].remotePort, "UDP")
		}
	}
}

func (this *Upnp) GetAllMapping() map[string][]PortMapping {
	return this.MappingPort.GetAllMapping()
}
