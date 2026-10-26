package defaultrpc

import "github.com/nats-io/nats.go"

// subscribeNatsReady 等待 NATS 确认已处理订阅，避免首次请求或回包早于订阅生效。
func subscribeNatsReady(conn *nats.Conn, subject string) (*nats.Subscription, error) {
	sub, err := conn.SubscribeSync(subject)
	if err != nil {
		return nil, err
	}
	if err = conn.FlushTimeout(nats.DefaultTimeout); err != nil {
		_ = sub.Unsubscribe()
		return nil, err
	}
	return sub, nil
}
