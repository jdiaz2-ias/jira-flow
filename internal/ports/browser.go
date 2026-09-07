package ports

import "context"

type Browser interface {
	Open(context.Context, string) error
}
