package scheduler

type Hooks struct {
	OnStart   func(ctrl FnControl) error
	OnStop    func(ctrl FnControl) error
	OnError   func(ctrl FnControl, err error)
	OnSuccess func(ctrl FnControl) error
	OnPause   func(ctrl FnControl) error
	OnResume  func(ctrl FnControl) error
}
