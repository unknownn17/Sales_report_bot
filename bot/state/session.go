package state

import (
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Flow int

const (
	FlowNone Flow = iota
	FlowAddProduct
	FlowConfirmSale
	FlowRecordPayment
	FlowCustomReport
	FlowCategoryManage
)

// AddProduct flow steps
const (
	AddStepWaitingExcel = iota + 1
)

// ConfirmSale flow steps
const (
	SaleStepSelectProduct = iota + 1
	SaleStepSelectSize
	SaleStepAskQuantity
	SaleStepAskPaymentType
	SaleStepAskBuyerName
	SaleStepAskBuyerContact
	SaleStepAskAmountPaid
	SaleStepAskDueDate
	SaleStepConfirm
)

// RecordPayment flow steps
const (
	PaymentStepEnterAmount = iota + 1
	PaymentStepEnterNote
)

// CustomReport flow steps
const (
	ReportStepEnterStartDate = iota + 1
	ReportStepEnterEndDate
)

// CategoryManage flow steps
const (
	CategoryStepEnterName = iota + 1
)

type Session struct {
	Flow Flow
	Step int
	Data map[string]interface{}
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[int64]*Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[int64]*Session),
	}
}

func (m *Manager) GetSession(adminID int64) *Session {
	m.mu.RLock()
	s, ok := m.sessions[adminID]
	m.mu.RUnlock()
	if ok {
		return s
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[adminID]; ok {
		return s
	}
	s = &Session{
		Flow: FlowNone,
		Step: 0,
		Data: make(map[string]interface{}),
	}
	m.sessions[adminID] = s
	return s
}

func (m *Manager) SetFlow(adminID int64, flow Flow, step int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[adminID]
	if !ok {
		s = &Session{Data: make(map[string]interface{})}
		m.sessions[adminID] = s
	}
	s.Flow = flow
	s.Step = step
}

func (m *Manager) SetStep(adminID int64, step int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[adminID]; ok {
		s.Step = step
	}
}

func (m *Manager) SetData(adminID int64, key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[adminID]
	if !ok {
		s = &Session{Data: make(map[string]interface{})}
		m.sessions[adminID] = s
	}
	s.Data[key] = value
}

func (m *Manager) GetData(adminID int64, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.sessions[adminID]; ok {
		v, exists := s.Data[key]
		return v, exists
	}
	return nil, false
}

func (m *Manager) GetDataString(adminID int64, key string) string {
	v, ok := m.GetData(adminID, key)
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (m *Manager) GetDataFloat(adminID int64, key string) float64 {
	v, ok := m.GetData(adminID, key)
	if !ok || v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func (m *Manager) GetDataInt(adminID int64, key string) int {
	v, ok := m.GetData(adminID, key)
	if !ok || v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

func (m *Manager) GetDataBool(adminID int64, key string) bool {
	v, ok := m.GetData(adminID, key)
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func (m *Manager) GetDataObjectID(adminID int64, key string) primitive.ObjectID {
	v, ok := m.GetData(adminID, key)
	if !ok || v == nil {
		return primitive.NilObjectID
	}
	if oid, ok := v.(primitive.ObjectID); ok {
		return oid
	}
	return primitive.NilObjectID
}

func (m *Manager) ClearSession(adminID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, adminID)
}

func (m *Manager) GetStockItems(adminID int64) []map[string]interface{} {
	v, ok := m.GetData(adminID, "stock_items")
	if !ok || v == nil {
		return nil
	}
	if items, ok := v.([]map[string]interface{}); ok {
		return items
	}
	return nil
}

func (m *Manager) GetSizes(adminID int64) []string {
	v, ok := m.GetData(adminID, "sizes")
	if !ok || v == nil {
		return nil
	}
	if sizes, ok := v.([]string); ok {
		return sizes
	}
	return nil
}
