package aigen

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Remote 是"额度记在别人家账本上"的那一半。
//
// 端口定在这里而不是直接引用 internal/picbi,是为了让依赖只有一个方向:
// picbi 包不认识 aigen,aigen 也不 import picbi,两者靠方法签名对上。参数
// 全是基础类型,正是为了不用共享一组 DTO —— 共享 DTO 就得有人 import 谁。
//
// 金额不出现在 Spend 的入参里:该扣几点由 pic.bi 按 (model, resolution) 自己
// 算。这不是接口洁癖,是定价权的归属问题。
type Remote interface {
	Enabled() bool
	Balance(ctx context.Context, picbiUserID string) (int, error)
	Quote(ctx context.Context, picbiUserID, model, resolution string) (int, error)
	Spend(ctx context.Context, picbiUserID, model, resolution, idempotencyKey, reason string) (opID string, credits int, err error)
	Refund(ctx context.Context, opID, idempotencyKey string) error
}

// remote 是进程级的那一个实例,启动时由 main 装好,之后只读。
//
// 用包级变量而不是给 Begin/Refund 多加一个参数:分流必须发生在这个包内部。
// 一旦让 handler 传进来,handler 就得先判断"这个人该走哪条账本",而那个判断
// 分散到调用点的第一天,就会有某处忘了判、或者判得和别处不一样。
var remote Remote

// SetRemote 在启动时调用一次。传 nil 表示这个部署没接 pic.bi。
func SetRemote(r Remote) { remote = r }

// RemoteEnabled 供接口层决定要不要给这个部署开放 pic.bi 相关的能力。
func RemoteEnabled() bool { return remote != nil && remote.Enabled() }

// RemoteBalance 查关联用户在 pic.bi 那边还剩多少。仅供展示。
func RemoteBalance(ctx context.Context, picbiUserID string) (int, error) {
	if !RemoteEnabled() || picbiUserID == "" {
		return 0, ErrRemoteUnavailable
	}
	n, err := remote.Balance(ctx, picbiUserID)
	if err != nil {
		return 0, mapRemoteErr(err)
	}
	return n, nil
}

var (
	// ErrRemoteUnavailable 是"这一笔的结果未知"。
	//
	// 它必须与 ErrNoCredits 分得清清楚楚:接口层看到它要回 503,绝不能退回
	// 本地免费额度。降级听起来体贴,实际是 pic.bi 每抖一下全站就能免费刷
	// 一轮 4k。
	ErrRemoteUnavailable = errors.New("aigen: pic.bi 暂时不可达")
	// ErrRemoteDenied 是对端明确拒绝:授权被撤销、账号非 active、参数不合法。
	ErrRemoteDenied = errors.New("aigen: pic.bi 拒绝了这次扣费")
)

// remoteKinder 是 picbi.Error 那一侧实现的方法集。用行为而不是类型来识别,
// 依赖就不必反过来指向 picbi。
type remoteKinder interface{ RemoteKind() string }

// mapRemoteErr 把客户端的错误折成本包的语义。
//
// default 落在"不可达"而不是"被拒绝"是有意的:认不出来的错误意味着我们不知道
// 那边发生了什么,而"不知道"在钱的问题上只能按最保守的方式处理——挡住,报
// 503,不放行。
func mapRemoteErr(err error) error {
	if err == nil {
		return nil
	}
	var k remoteKinder
	if errors.As(err, &k) {
		switch k.RemoteKind() {
		case "no_credits":
			return fmt.Errorf("%w: %v", ErrNoCredits, err)
		case "forbidden", "invalid", "conflict":
			return fmt.Errorf("%w: %v", ErrRemoteDenied, err)
		}
	}
	return fmt.Errorf("%w: %v", ErrRemoteUnavailable, err)
}

// remoteTimeout 卡住扣费/退款那一次 HTTP。
//
// 比上游生成的超时短得多:扣费只是一次记账调用,拖到几十秒说明对端已经不行
// 了,继续等只会把用户的请求一起挂死。
const remoteTimeout = 10 * time.Second

func remoteCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), remoteTimeout)
}

// spendKey / refundKey 由生成记录的 ID 确定性地导出。
//
// 幂等键必须是"这件事"的名字,不能是随机串:重试时算出来的键要和上一次一模
// 一样,pic.bi 才认得出这是同一笔而不是新的一笔。前缀区分扣费与退款,免得
// 同一条记录的两个动作撞在同一个键上。
func spendKey(genID fmt.Stringer) string  { return "openimg:gen:" + genID.String() }
func refundKey(genID fmt.Stringer) string { return "openimg:refund:" + genID.String() }
