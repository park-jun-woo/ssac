package generator

import (
	"testing"

	"github.com/park-jun-woo/ssac/parser"
)

func TestGenerateSubscribeFunc(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "OnOrderCompleted", FileName: "on_order_completed.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "OnOrderCompletedMessage"},
		Param:     &parser.ParamInfo{TypeName: "OnOrderCompletedMessage", VarName: "message"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqPut, Model: "Order.UpdateNotified", Inputs: map[string]string{"ID": "order.ID", "Notified": `"true"`}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "func (h *Handler) OnOrderCompleted(ctx context.Context, message OnOrderCompletedMessage) error {")
	assertContains(t, code, "return nil")
	assertContains(t, code, `return fmt.Errorf(`)
	assertContains(t, code, `"context"`)
	assertContains(t, code, `"fmt"`)
	assertNotContains(t, code, "gin.Context")
	assertNotContains(t, code, "c.JSON")
}

func TestGenerateSubscribeGet(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "OnTest", FileName: "on_test.go",
		Subscribe: &parser.SubscribeInfo{Topic: "test", MessageType: "TestMsg"},
		Param:     &parser.ParamInfo{TypeName: "TestMsg", VarName: "message"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqEmpty, Target: "order", Message: "주문을 찾을 수 없습니다"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "h.OrderModel.FindByID(message.OrderID)")
	assertContains(t, code, `return fmt.Errorf("Order 조회 실패: %w", err)`)
	assertContains(t, code, `return fmt.Errorf("주문을 찾을 수 없습니다")`)
}

func TestGenerateSubscribePublish(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "OnOrderCompleted", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "message"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqPublish, Topic: "notification.send", Inputs: map[string]string{"Email": "message.Email"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `queue.Publish(ctx, "notification.send"`)
	assertNotContains(t, code, "c.Request.Context()")
	assertContains(t, code, `"queue"`)
}

func TestGenerateSubscribeEmpty(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "OnTest", FileName: "on_test.go",
		Subscribe: &parser.SubscribeInfo{Topic: "test", MessageType: "TestMsg"},
		Param:     &parser.ParamInfo{TypeName: "TestMsg", VarName: "message"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"ID": "message.UserID"}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqEmpty, Target: "user", Message: "사용자 없음"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `return fmt.Errorf("사용자 없음")`)
	assertNotContains(t, code, "c.JSON(http.StatusNotFound")
}

func TestGenerateSubscribeAuth(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "OnTest", FileName: "on_test.go",
		Subscribe: &parser.SubscribeInfo{Topic: "test", MessageType: "TestMsg"},
		Param:     &parser.ParamInfo{TypeName: "TestMsg", VarName: "message"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "process", Resource: "order", Inputs: map[string]string{"OrderID": "message.OrderID"}, Message: "Not authorized"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `authz.Check(authz.CheckRequest{Action: "process", Resource: "order"`)
	assertContains(t, code, `return fmt.Errorf("Not authorized: %w", err)`)
	assertNotContains(t, code, "c.JSON")
}
