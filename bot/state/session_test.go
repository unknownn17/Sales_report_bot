package state

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSessionManager(t *testing.T) {
	mgr := NewManager()

	admin1 := int64(1001)
	admin2 := int64(2002)

	mgr.SetFlow(admin1, FlowAddProduct, AddStepWaitingExcel)
	mgr.SetData(admin1, "name", "Pijama Silk")
	mgr.SetData(admin1, "sell_price", 250000.0)

	mgr.SetFlow(admin2, FlowConfirmSale, SaleStepAskQuantity)
	mgr.SetData(admin2, "sale_qty", 3)

	sess1 := mgr.GetSession(admin1)
	sess2 := mgr.GetSession(admin2)

	if sess1.Flow != FlowAddProduct || sess1.Step != AddStepWaitingExcel {
		t.Errorf("admin1 session state mismatch")
	}

	if mgr.GetDataString(admin1, "name") != "Pijama Silk" {
		t.Errorf("admin1 name mismatch")
	}

	if mgr.GetDataFloat(admin1, "sell_price") != 250000.0 {
		t.Errorf("admin1 price mismatch")
	}

	if sess2.Flow != FlowConfirmSale || sess2.Step != SaleStepAskQuantity {
		t.Errorf("admin2 session state mismatch")
	}

	if mgr.GetDataInt(admin2, "sale_qty") != 3 {
		t.Errorf("admin2 qty mismatch")
	}

	// Test ObjectID data
	oid := primitive.NewObjectID()
	mgr.SetData(admin1, "obj_id", oid)
	if mgr.GetDataObjectID(admin1, "obj_id") != oid {
		t.Errorf("ObjectID mismatch")
	}

	// Test clear session
	mgr.ClearSession(admin1)
	clearedSess := mgr.GetSession(admin1)
	if clearedSess.Flow != FlowNone || clearedSess.Step != 0 {
		t.Errorf("Session was not cleared properly")
	}
}
