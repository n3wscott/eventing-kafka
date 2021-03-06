/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consumer

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

type KafkaConsumerHandler interface {
	// When this function returns true, the consumer group offset is marked as consumed.
	// The returned error is enqueued in errors channel.
	Handle(context context.Context, message *sarama.ConsumerMessage) (bool, error)
	SetReady(partition int32, ready bool)
	GetConsumerGroup() string
}

type SaramaConsumerLifecycleListener interface {
	// Setup is invoked when the consumer is joining the session
	Setup(sess sarama.ConsumerGroupSession)

	// Cleanup is invoked when the consumer is leaving the session
	Cleanup(sess sarama.ConsumerGroupSession)
}

type noopSaramaConsumerLifecycleListener struct{}

func (n noopSaramaConsumerLifecycleListener) Setup(sess sarama.ConsumerGroupSession) {}

func (n noopSaramaConsumerLifecycleListener) Cleanup(sess sarama.ConsumerGroupSession) {}

func WithSaramaConsumerLifecycleListener(listener SaramaConsumerLifecycleListener) SaramaConsumerHandlerOption {
	return func(handler *SaramaConsumerHandler) {
		handler.lifecycleListener = listener
	}
}

// ConsumerHandler implements sarama.ConsumerGroupHandler and provides some glue code to simplify message handling
// You must implement KafkaConsumerHandler and create a new SaramaConsumerHandler with it
type SaramaConsumerHandler struct {
	// The user message handler
	handler KafkaConsumerHandler

	lifecycleListener SaramaConsumerLifecycleListener

	logger *zap.SugaredLogger

	// Errors channel
	errors chan error
}

type SaramaConsumerHandlerOption func(*SaramaConsumerHandler)

func NewConsumerHandler(logger *zap.SugaredLogger, handler KafkaConsumerHandler, errorsCh chan error, options ...SaramaConsumerHandlerOption) SaramaConsumerHandler {
	sch := SaramaConsumerHandler{
		handler:           handler,
		lifecycleListener: noopSaramaConsumerLifecycleListener{},
		logger:            logger,
		errors:            errorsCh,
	}

	for _, f := range options {
		f(&sch)
	}

	return sch
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *SaramaConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	consumer.logger.Info("setting up handler")
	consumer.lifecycleListener.Setup(session)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *SaramaConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	consumer.logger.Infow("Cleanup handler")
	for t, ps := range session.Claims() {
		for _, p := range ps {
			consumer.logger.Debugw("Cleanup handler: Setting partition readiness to false", zap.String("topic", t),
				zap.Int32("partition", p))
			consumer.handler.SetReady(p, false)
		}
	}
	consumer.lifecycleListener.Cleanup(session)
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *SaramaConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	consumer.logger.Infow(fmt.Sprintf("Starting partition consumer, topic: %s, partition: %d, initialOffset: %d", claim.Topic(), claim.Partition(), claim.InitialOffset()), zap.String("ConsumeGroup", consumer.handler.GetConsumerGroup()))
	consumer.handler.SetReady(claim.Partition(), true)
	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {

		if ce := consumer.logger.Desugar().Check(zap.DebugLevel, "debugging"); ce != nil {
			consumer.logger.Debugw("Message claimed", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
		}

		// Don't use the session context since it is closed before messages are drained.
		// Handle must finish before the session timeout.
		mustMark, err := consumer.handler.Handle(context.Background(), message)

		if err != nil {
			consumer.logger.Infow("Failure while handling a message", zap.String("topic", message.Topic), zap.Int32("partition", message.Partition), zap.Int64("offset", message.Offset), zap.Error(err))
			consumer.errors <- err
			consumer.handler.SetReady(claim.Partition(), false)
		}

		if mustMark {
			session.MarkMessage(message, "") // Mark kafka message as processed
			if ce := consumer.logger.Desugar().Check(zap.DebugLevel, "debugging"); ce != nil {
				consumer.logger.Debugw("Message marked", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
			}
		}

	}

	consumer.logger.Infof("Stopping partition consumer, topic: %s, partition: %d", claim.Topic(), claim.Partition())
	return nil
}

var _ sarama.ConsumerGroupHandler = (*SaramaConsumerHandler)(nil)
