package amqputil

import "testing"

func init() {

}

func TestConnectAmqp(t *testing.T) {
	conf := AmqpConnectionHelper{AmqpConnectionURL: "amqp://guest:guest@cicd_rabbitmq_1:5672/"}

	amqpContext := conf.GetAmqpContext("test1")
	if amqpContext == nil {
		t.Fatal("Couldn't connect to messaging server")
	}

	amqpContext.Close()
}
