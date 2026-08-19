package api

import "testing"

// 路由冲突在 gin 里是注册时 panic,而不是编译错误——/api/tokens/current 与
// /api/tokens/:id 同层,静态段与通配符撞车的话整个服务起不来。这条断言让它
// 在测试里就炸,而不是在生产启动时。
func TestRouterRegistersWithoutConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("路由注册 panic: %v", r)
		}
	}()
	s := &Server{}
	_ = s.Router()
}
