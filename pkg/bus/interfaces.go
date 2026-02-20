package bus

import "context"

type Publisher interface {
	PublishInbound(InboundMessage)
	PublishOutbound(OutboundMessage)
}

type Subscriber interface {
	ConsumeInbound(context.Context) (InboundMessage, bool)
	SubscribeOutbound(context.Context) (OutboundMessage, bool)
}

type Broker interface {
	Publisher
	Subscriber
	RegisterHandler(channel string, handler MessageHandler)
	GetHandler(channel string) (MessageHandler, bool)
	Close()
}
