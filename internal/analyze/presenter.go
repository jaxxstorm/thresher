package analyze

import "context"

type Presenter interface {
	Run(context.Context, *StateStore, func(context.Context) error) error
}
