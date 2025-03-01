package sip

import (
	"fmt"
	"net"
	"sync"
)

var UEs *UserEquipments = NewUserEquipments()

type UserEquipment struct {
	Enabled       bool         `json:"enabled"`
	Imsi          string       `json:"imsi"`
	Ki            string       `json:"ki"`
	Opc           string       `json:"opc"`
	MsIsdn        string       `json:"msisdn"`
	RegStatus     string       `json:"regStatus"`
	Expires       string       `json:"expires"`
	UdpPort       int          `json:"udpPort"`
	UDPListener   *net.UDPConn `json:"-"`
	Authorization string       `json:"-"`
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
	ues.eqs[ue.Imsi] = ue
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
		delete(ues.eqs, imsi)
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

func (ues *UserEquipments) RegisterUE(imsi string) error {
	ues.mu.RLock()
	defer ues.mu.RUnlock()
	ue, ok := ues.eqs[imsi]
	if !ok {
		return fmt.Errorf("UE not found")
	}
	go RegisterMe(ue, "")
	return nil
}

func (ues *UserEquipments) CallViaUE(imsi, cdpn string) error {
	ues.mu.RLock()
	defer ues.mu.RUnlock()
	ue, ok := ues.eqs[imsi]
	if !ok {
		return fmt.Errorf("UE not found")
	}
	go CallViaUE(ue, cdpn)
	return nil
}
