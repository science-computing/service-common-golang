// Package amqputil provides AmqpContext to simplify AMQP interaction
package amqputil

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/science-computing/service-common-golang/apputil"

	"github.com/pkg/errors"
)

var log = apputil.InitLogging()

type AmqpAccessor interface {
	PublishMessage(queueName string, message interface{}) error
	ReceiveMessage(queueName string, message interface{}) (delivery *amqp.Delivery, err error)
	Channel() ChannelAccessor
	Close() error
	Reset() error
	LastError() error
	SetLastError(err error)
	ResetError()
}

// ChannelAccessor is an interface for the necessary methods to access the Channel struct of the AMQP library.
// the library does not define an interface, so we do it here (it helps for mocking)
// this interface only defines those methods that we know we need. See https://pkg.go.dev/github.com/rabbitmq/amqp091-go
// for all possible methods.
type ChannelAccessor interface {
	Qos(prefetchCount, prefetchSize int, global bool) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Close() error
	Cancel(consumer string, noWait bool) error
	QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error)
	QueueInspect(name string) (amqp.Queue, error)
}

// AmqpConnectionHelper helps to get a connection AMQP
type AmqpConnectionHelper struct {
	AmqpConnectionURL string
}

// AmqpContext simplifies amqp interaction by providing a context with
// a persistent connection and a channel to simplify message publishing
type AmqpContext struct {
	err     error
	channel ChannelAccessor

	connection        *amqp.Connection
	amqpConnectionURL string
	consumerId        string
	queues            map[string]amqp.Queue
	deliveryChannels  map[string]<-chan amqp.Delivery
}

// ErrNoMessages indicates, that no message were found in a queue
var ErrNoMessage = errors.Errorf("No message found in queue")

// GetAmqpContext creates an AmqpContext for the given amqpConnectionURL
// or returns an already existing AmqpContext for the amqpConnectionURL
// the consumerId identifies the consumer on the channel
func (helper *AmqpConnectionHelper) GetAmqpContext(consumerId string) (amqpContext *AmqpContext) {
	log.Debugf("Get AmqpContext for URL [%v] and id [%s]", helper.AmqpConnectionURL, consumerId)
	amqpContext = &AmqpContext{}
	amqpContext.amqpConnectionURL = helper.AmqpConnectionURL
	amqpContext.consumerId = consumerId
	log.Debugf("Opening AMQP connection to [%v]", helper.AmqpConnectionURL)
	// create connection
	if amqpContext.connection, amqpContext.err = amqp.Dial(helper.AmqpConnectionURL); amqpContext.err != nil {
		log.Warnf("Cannot open AMPQ connection to '%s', Reason: %s ", helper.AmqpConnectionURL, amqpContext.err.Error())
		return nil
	}

	// create channel
	amqpContext.Reset()

	return amqpContext
}

func (amqpContext *AmqpContext) Channel() ChannelAccessor {
	return amqpContext.channel
}

// Reset resets the channel and queues - asumes that
func (amqpContext *AmqpContext) Reset() error {
	if amqpContext.connection.IsClosed() {
		log.Debugf("Reopening connection to %s: ", amqpContext.amqpConnectionURL)
		if amqpContext.connection, amqpContext.err = amqp.Dial(amqpContext.amqpConnectionURL); amqpContext.err != nil {
			log.Warnf("Cannot open AMPQ context, Reason: %s ", amqpContext.err.Error())
			return amqpContext.err
		}
	}
	// create channel
	if amqpContext.channel, amqpContext.err = amqpContext.connection.Channel(); amqpContext.err != nil {
		log.Warnf("Cannot open AMPQ channel, Reason: %s ", amqpContext.err.Error())
		return amqpContext.err
	}

	if amqpContext.channel == nil {
		log.Error("Channel is nil this should not happen")
	}

	amqpContext.queues = make(map[string]amqp.Queue)
	amqpContext.deliveryChannels = make(map[string]<-chan amqp.Delivery)
	return amqpContext.err
}

func (amqpContext *AmqpContext) EnsureQueueExists(queueName string) error {
	// get queue from internal map or create new one
	_, ok := amqpContext.queues[queueName]
	if !ok {
		var args = make(amqp.Table)
		// args["x-queue-mode"] = "lazy"
		amqpContext.queues[queueName], amqpContext.err =
			amqpContext.channel.QueueDeclare(queueName, false, false, false, false, args)
		if amqpContext.err != nil {
			amqpContext.err = errors.Wrapf(amqpContext.err, "Cannot declare AMQP queue [%v]", queueName)
			return amqpContext.err
		}
	}
	return nil
}

// PublishMessage sends given message as application/json to queue with given name.
// If the queue does not exist, it is created.
// Errors go to AmqpContext.Err
func (amqpContext *AmqpContext) PublishMessage(queueName string, message interface{}) error {
	log.Debugf("Publising message [%v] to queue [%v]", message, queueName)

	// get queue from internal map or create new one
	amqpContext.err = amqpContext.EnsureQueueExists(queueName)
	if amqpContext.err != nil {
		return amqpContext.err
	}

	body, err := json.Marshal(message)
	if err != nil {
		amqpContext.err = errors.Wrapf(err, "Failed to marshall AMQP message [%v]", message)
		return amqpContext.err
	}

	log.Debugf("Publishing message [%v] to AMQP", string(body))
	publishing := amqp.Publishing{ContentType: "application/json", Body: body}
	// publish to default exchange ""
	if err = amqpContext.channel.Publish("", queueName, false, false, publishing); err != nil {
		amqpContext.err = errors.Wrapf(err, "Failed to publish AMQP message [%v]", message)
		return amqpContext.err
	}
	return amqpContext.err
}

func (amqpContext *AmqpContext) registerConsumer(queueName string) {
	var deliveryChan <-chan amqp.Delivery
	retries := 0
	amqpContext.err = amqpContext.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	for amqpContext.err != nil && retries < 10 {
		retries++
		log.Warnf("Queue %s ist not available, retrying in 3s: %v", queueName, amqpContext.err)
		time.Sleep(3 * time.Second)
		amqpContext.Reset()
		if amqpContext.err != nil {
			continue
		}
		amqpContext.err = amqpContext.channel.Qos(
			1,     // prefetch count
			0,     // prefetch size
			false, // global
		)
	}
	if amqpContext.err != nil {
		amqpContext.err = errors.Wrapf(amqpContext.err, "Failed to set Qos on queue [%v] for consumerId [%v]", queueName, amqpContext.consumerId)
		return
	}

	log.Debugf("Registering consumer [%v] on queue [%v]", amqpContext.consumerId, queueName)
	retries = 0
	for {
		deliveryChan, amqpContext.err = amqpContext.channel.Consume(queueName, amqpContext.consumerId, false, false, false, false, nil)
		if amqpContext.err != nil {
			// if queue was not found, retry
			if notFoundError, ok := amqpContext.err.(*amqp.Error); ok && notFoundError.Code == 404 && retries < 10 {
				log.Debugf("Consumer %v did not find queue [%v]. Retrying", amqpContext.consumerId, queueName)
				// necessary as Consume() leads to a "channel not open" error after first timed out attempt
				amqpContext.Reset()
				retries++
				time.Sleep(3 * time.Second)
			} else {
				// if there was another error
				amqpContext.err = errors.Wrapf(amqpContext.err, "Cannot consume AMQP queue [%v] for consumerId [%v]", queueName, amqpContext.consumerId)
				return
			}
		} else {
			amqpContext.err = nil
			break
		}
	}
	amqpContext.deliveryChannels[queueName] = deliveryChan
}

// ReceiveMessage gets next message from queue with given queue name
func (amqpContext *AmqpContext) ReceiveMessage(queueName string, message interface{}) (delivery *amqp.Delivery, err error) {
	log.Debugf("Receiving message from queue [%v] for consumerId [%v)", queueName, amqpContext.consumerId)

	// get delivery from internal map or create new one
	deliveryChan := amqpContext.deliveryChannels[queueName]
	if deliveryChan == nil {
		amqpContext.registerConsumer(queueName)
		if amqpContext.err != nil {
			log.Errorf("Unable to register consumer %v", amqpContext.err)
			return nil, amqpContext.err
		}
		deliveryChan = amqpContext.deliveryChannels[queueName]
	}

	var retDelivery amqp.Delivery
	var ok bool

	// return false after timeout or non-ok channel read
	select {
	case <-time.After(10 * time.Second):
		amqpContext.err = ErrNoMessage
		log.Debugf("No message delivered for consumerId [%v].", amqpContext.consumerId)
		// stop consuming
		amqpContext.channel.Cancel(amqpContext.consumerId, false)
		return nil, amqpContext.err
	case retDelivery, ok = <-deliveryChan:
		if ok && (retDelivery.Body == nil || len(retDelivery.Body) == 0) {
			amqpContext.err = errors.New("Failed to get delivery from delivery chan. Body is empty. ConsumerId [" + amqpContext.consumerId + "]")
			return nil, amqpContext.err
		} else if !ok {
			// chan is closed -> remove consumer
			log.Debugf("Chan is closed for consumerId [%v]. ", amqpContext.consumerId)
			amqpContext.channel.Cancel(amqpContext.consumerId, false)

			/*err := amqpContext.registerConsumer(queueName)
			if err != nil {
				log.Errorf("Unable to register consumer %v", err)
			}*/

			return nil, amqpContext.err
		}
	}

	// unmarshal delivery
	amqpContext.err = json.Unmarshal(retDelivery.Body, message)

	return &retDelivery, amqpContext.err
}

// ReceiveMessage gets next message from queue with given queue name
func (amqpContext *AmqpContext) ReceiveProtoMessage(queueName string, message proto.Message) (delivery *amqp.Delivery, err error) {
	log.Debugf("Receiving message from queue [%v] for consumerId [%v)", queueName, amqpContext.consumerId)

	// get delivery from internal map or create new one
	deliveryChan := amqpContext.deliveryChannels[queueName]
	if deliveryChan == nil {
		amqpContext.registerConsumer(queueName)
		if amqpContext.err != nil {
			log.Errorf("Unable to register consumer %v", amqpContext.err)
			return nil, amqpContext.err
		}
		deliveryChan = amqpContext.deliveryChannels[queueName]
	}

	var retDelivery amqp.Delivery
	var ok bool

	// return false after timeout or non-ok channel read
	select {
	case <-time.After(10 * time.Second):
		amqpContext.err = ErrNoMessage
		log.Debugf("No message delivered for consumerId [%v].", amqpContext.consumerId)
		// stop consuming
		amqpContext.channel.Cancel(amqpContext.consumerId, false)
		return nil, amqpContext.err
	case retDelivery, ok = <-deliveryChan:
		if ok && (retDelivery.Body == nil || len(retDelivery.Body) == 0) {
			amqpContext.err = errors.New("Failed to get delivery from delivery chan. Body is empty. ConsumerId [" + amqpContext.consumerId + "]")
			return nil, amqpContext.err
		} else if !ok {
			// chan is closed -> remove consumer
			log.Debugf("Chan is closed for consumerId [%v]. ", amqpContext.consumerId)
			amqpContext.channel.Cancel(amqpContext.consumerId, false)

			/*err := amqpContext.registerConsumer(queueName)
			if err != nil {
				log.Errorf("Unable to register consumer %v", err)
			}*/

			return nil, amqpContext.err
		}
	}

	// unmarshal delivery
	amqpContext.err = protojson.Unmarshal(retDelivery.Body, message)

	return &retDelivery, amqpContext.err
}

// Close closes the amqp connection
func (amqpContext *AmqpContext) Close() error {
	log.Info("Closing AMQP connection and channel")
	if amqpContext.channel != nil {
		amqpContext.channel.Close()
	}
	amqpContext.err = amqpContext.connection.Close()
	return amqpContext.err
}

func (amqpContext *AmqpContext) LastError() error {
	return amqpContext.err
}

func (amqpContext *AmqpContext) ResetError() {
	amqpContext.err = nil
}

func (amqpContext *AmqpContext) SetLastError(err error) {
	amqpContext.err = err
}
