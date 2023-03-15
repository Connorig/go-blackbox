package awesomeProject1

import "context"

type application struct {
	builder *applicationStart
}

func New() (app *application) {
	builder := &applicationStart{}

	app = &application{
		builder,
	}
	return
}

func (application *application) Start(builder func(ctx context.Context, builder *applicationStart) error, onTerminate ...func(string)) (err error) {

	err = builder(nil, application.builder)
	if err != nil {
	}
	return
}
