package application

import "context"

type EventConsumer interface {
	Start(ctx context.Context) error
}
