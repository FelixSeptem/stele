package jobs

type NoopWorker struct{}

func (NoopWorker) Start() error {
	return nil
}

type NoopScheduler struct{}

func (NoopScheduler) Start() error {
	return nil
}
