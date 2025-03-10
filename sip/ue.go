package sip

import (
	"fmt"
	"net"
	"sipclientgo/global"
	"sipclientgo/system"
	"sync"
)

var UEs *UserEquipments = NewUserEquipments()

type SessionsMap = *ConcurrentMapMutex[SipSession]

type UserEquipment struct {
	Enabled       bool        `json:"enabled"`
	Imsi          string      `json:"imsi"`
	Ki            string      `json:"ki"`
	Opc           string      `json:"opc"`
	MsIsdn        string      `json:"msisdn"`
	RegStatus     string      `json:"regStatus"`
	Expires       string      `json:"expires"`
	UdpPort       int         `json:"udpPort"`
	Authorization string      `json:"-"`
	SesMap        SessionsMap `json:"-"`

	UDPListener *net.UDPConn `json:"-"`
	DataChan    chan Packet  `json:"-"`
}

type UserEquipments struct {
	mu  sync.RWMutex
	eqs map[string]*UserEquipment
}

func NewUserEquipments() *UserEquipments {
	return &UserEquipments{
		eqs: make(map[string]*UserEquipment),
	}
}

func (ues *UserEquipments) AddUE(ue *UserEquipment) error {
	ues.mu.Lock()
	defer ues.mu.Unlock()
	if _, ok := ues.eqs[ue.Imsi]; ok {
		return fmt.Errorf("UE already exists")
	}
	for k, v := range ues.eqs {
		if ue.UdpPort == v.UdpPort {
			return fmt.Errorf("UDP port already in use with UE: %s", k)
		}
	}
	err := StartUEListener(ue)
	if err != nil {
		return err
	}
	ue.SesMap = NewConcurrentMapMutex[SipSession]()
	ues.eqs[ue.Imsi] = ue
	system.LogInfo(system.LTRegistration, fmt.Sprintf("New UE started on [%s:%d]", global.ClientIPv4.String(), ue.UdpPort))
	return nil
}

func (ues *UserEquipments) GetUE(imsi string) *UserEquipment {
	ues.mu.RLock()
	defer ues.mu.RUnlock()
	return ues.eqs[imsi]
}

func (ues *UserEquipments) DeleteUEs(imsis ...string) {
	if len(imsis) == 0 {
		return
	}
	ues.mu.Lock()
	defer ues.mu.Unlock()
	for _, imsi := range imsis {
		if ue, ok := ues.eqs[imsi]; ok {
			if ue.DataChan != nil {
				close(ue.DataChan)
			}
			if ue.UDPListener != nil {
				ue.UDPListener.Close()
			}
			delete(ues.eqs, imsi)
		}
	}
}

func (ues *UserEquipments) GetUEs() []*UserEquipment {
	ues.mu.RLock()
	defer ues.mu.RUnlock()
	uesList := make([]*UserEquipment, 0, len(ues.eqs))
	for _, ue := range ues.eqs {
		uesList = append(uesList, ue)
	}
	return uesList
}

func (ues *UserEquipments) DoRegister(imsi string, unreg bool) error {
	ues.mu.RLock()
	defer ues.mu.RUnlock()
	ue, ok := ues.eqs[imsi]
	if !ok {
		return fmt.Errorf("UE not found")
	}
	if unreg {
		go UnregisterMe(ue, "")
	} else {
		go RegisterMe(ue, "")
	}
	return nil
}

func (ues *UserEquipments) DoCall(imsi, cdpn string) error {
	ues.mu.RLock()
	defer ues.mu.RUnlock()
	ue, ok := ues.eqs[imsi]
	if !ok {
		return fmt.Errorf("UE not found")
	}
	if cdpn == "" {
		return fmt.Errorf("invalid CDPN")
	}
	go CallViaUE(ue, cdpn)
	return nil
}
